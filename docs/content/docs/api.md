---
title: HTTP API
weight: 4
---

A chi router with `RequestID`, `RealIP`, request logging, `Recoverer`, a
`/ping` heartbeat, and the metrics middleware applied globally.

| Endpoint | Auth | Rate limited | Purpose |
|---|---|---|---|
| `GET /ping` | no | no | Heartbeat. Returns `.`. Used by the image's healthcheck. |
| `GET /version` | no | no | Build info as plain text. |
| `GET /metrics` | no | no | Prometheus exposition. |
| `POST /v1/update/{serviceName}` | **yes** | **yes** | Change a running service's image. |

## Why /metrics is public

Auth and the rate limiter are applied to the `/v1` group only, deliberately. If
they wrapped `/metrics`, every Prometheus scrape would 401 (inflating
`swarmctl_auth_failures_total`) and burn the per-IP rate-limit budget
(inflating `swarmctl_rate_limited_requests_total` and dropping scrapes).

Consequence: anyone who can reach the port can read your metrics and version.
Publish the hostname behind the tunnel — and behind an Access policy if that
matters to you.

## Authentication

```
Authorization: Bearer <AUTH_TOKEN>
```

The prefix `Bearer ` must be present exactly; the remainder is compared to
`AUTH_TOKEN` with `crypto/subtle.ConstantTimeCompare`. A missing prefix or a
mismatch increments `swarmctl_auth_failures_total` and returns `401
Unauthorized` with no body detail.

## Rate limit

10 requests per minute per client IP across the `/v1` group
(`httprate`, keyed on the `RealIP`-resolved address). Over the limit returns
`429 Too many requests` and increments
`swarmctl_rate_limited_requests_total`. Both 401 and 429 responses are still
counted in `swarmctl_http_requests_total`, because the metrics middleware wraps
auth and the limiter.

## POST /v1/update/{serviceName}

```bash
curl -X POST -H "Authorization: Bearer $AUTH_TOKEN" \
  "https://swarmctl.example.com/v1/update/web_server?image=ghcr.io/you/web:sha-abc123"
```

| Parameter | In | Notes |
|---|---|---|
| `serviceName` | path | Swarm service name, e.g. `stack_service`. |
| `image` | query | Full image reference including tag or digest. |

Both are required; either missing gives `400`.

Success is `200` with:

```json
{ "success": true, "oldVersion": 412, "newVersion": 413 }
```

`oldVersion` / `newVersion` are the service's Swarm spec version indexes before
and after the update — a cheap way for CI to assert something actually changed.

Failure is `500` with the Docker error as the body (unknown service, bad image
reference, socket problems).

### What it changes

The handler inspects the service, sets **only**
`Spec.TaskTemplate.ContainerSpec.Image`, and re-submits the existing spec with
the version it just read. Replicas, mounts, networks, env, and labels are
carried through unchanged. Swarm then rolls the service according to whatever
update policy the service already has — swarmctl does not wait for convergence
and returns as soon as the API accepts the update.

{{< callout type="warning" >}}
Because the version index is read and re-submitted immediately, two concurrent
updates to the same service can race: the second one fails with an
`update out of sequence` error from Docker. Retry, don't ignore.
{{< /callout >}}

Every call records `swarmctl_docker_service_updates_total{service_name,status}`
and the corresponding duration histogram.

## GET /version

```
Go Version: go1.24.13
Version: v1.4.0
Commit: e1ec658
Build Time: Thu Jul 24 20:15:00 2026
OS/Arch: linux/amd64
```

Values come from `-ldflags -X` at build time, falling back to the module's
embedded VCS build info, then to `devel` / `unknown`.
