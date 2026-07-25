---
title: Observability
weight: 6
---

## Metrics

`GET /metrics` serves the Prometheus exposition format and is **not** behind
auth or the rate limiter (see [HTTP API](../api/)).

| Metric | Type | Labels |
|---|---|---|
| `swarmctl_http_requests_total` | counter | `method`, `endpoint`, `status_code` |
| `swarmctl_http_request_duration_seconds` | histogram | `method`, `endpoint` |
| `swarmctl_docker_service_updates_total` | counter | `service_name`, `status` (`success`/`error`) |
| `swarmctl_docker_service_update_duration_seconds` | histogram | `service_name` |
| `swarmctl_docker_events_total` | counter | `event_type`, `service_name` |
| `swarmctl_cloudflare_sync_total` | counter | `status` (`success`/`error`) |
| `swarmctl_cloudflare_sync_duration_seconds` | histogram | — |
| `swarmctl_pushover_notifications_total` | counter | `status` |
| `swarmctl_auth_failures_total` | counter | — |
| `swarmctl_rate_limited_requests_total` | counter | — |
| `swarmctl_active_connections` | gauge | — |
| `swarmctl_recent_events_count` | gauge | — |

Two details that matter when reading the numbers:

- The `/metrics` request itself is **not** counted in the HTTP metrics. At a
  15s scrape interval it would otherwise dominate the request rate.
- Unmatched routes report `endpoint="unmatched"` rather than the raw path, so
  scanner traffic cannot blow up label cardinality. `endpoint` is always the
  chi route *pattern* — `/v1/update/{serviceName}`, not the resolved name.
- `swarmctl_recent_events_count` is recalculated at scrape time, so it is only
  as fresh as your scrape interval.

## Scraping

Swarm's DNS gives you one A record per task, so scrape by DNS service
discovery. The repo ships this as `prometheus/prometheus.yml`:

```yaml
global:
  scrape_interval:     15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'swarmctl'
    metrics_path: /metrics
    dns_sd_configs:
      - names:
          - 'tasks.swarmctl_server'
        type: 'A'
        port: 9000
```

`tasks.<service>` is Swarm's per-task DNS name; adjust it if your stack or
service is not named `swarmctl_server`. Prometheus needs to be on a network
the swarmctl service is attached to.

## Grafana

A prebuilt dashboard lives in `grafana/dashboard/dashboard.json`, with
provisioning files next to it (`grafana/dashboard/dashboard.yml`,
`grafana/datasources/datasource.yml`). Mount the directory into the Grafana
container's provisioning path, or import the JSON by hand. A screenshot of what
it looks like is at `grafana/dashboard/example.png`.

## Alerts to your phone

Container `die` / `restart` / `crash` events go out as Pushover notifications to
`PUSHOVER_RECIPIENT`, titled `DOCKER SWARM EVENT`, with the container name,
short ID, and exit code. Delivery outcomes are counted in
`swarmctl_pushover_notifications_total{status}` — a rising `error` count means
the app key or recipient is wrong, and you are silently missing pushes.

Deduplication is per `containerID:action` with a one-minute cooldown, so a crash
loop is one push a minute. A container that is *replaced* on every restart (the
normal Swarm behaviour) has a new ID each time, so those are not deduplicated.

## Logs

Structured JSON on stdout (`docker service logs swarmctl_server`), plus a
Discord mirror of everything at `WARN` and above via `WEBHOOK_URL`. Every line
carries `env` and `service=swarmctl`; HTTP handler lines also carry
`requestID`, which chi generates per request.

Run with `-debug` to see the reconciler's decisions — sync attempts, ignored
services, dedup hits, pending removals, and each orphaned hostname removed.
Without it, those are silent.
