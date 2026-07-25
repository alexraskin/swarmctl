---
title: Troubleshooting
weight: 7
---

## It exits immediately at startup

```json
{"level":"ERROR","msg":"Environment variable is not set","key":"PUSHOVER_RECIPIENT"}
```

Every variable in [Configuration](../configuration/#required) is mandatory —
including the Pushover and Discord ones, even if you do not want either. The
process exits with `-1` on the first missing one. In Swarm this looks like a
task that restarts forever with no listener; check `docker service logs`.

If a value points at a Docker secret, remember that only a path that **exists**
is read as a file. A typo'd path is used as a literal string, and you find out
later when Cloudflare rejects the credential.

## A service has labels but nothing happens

Run through these in order:

1. **Service labels, not container labels.** They must be under
   `deploy.labels:` in compose. swarmctl reads `Service.Spec.Labels`.
   `docker service inspect <name> --format '{{ json .Spec.Labels }}'` shows what
   it actually sees.
2. **`enabled` must be exactly `true`.** `True`, `1`, and `yes` are all ignored.
3. **Something has to emit an event.** swarmctl syncs on `create` and `update`
   events only; it does not sweep existing services at startup. Adding labels to
   a running service is an update, so it fires — but a service that was already
   labelled before swarmctl started is not synced until it changes again. Force
   it with `docker service update --force <name>`.
4. **Look at the debug log.** `Service is not enabled for Cloudflare tunnel`
   means the labels did not match; `Cloudflare sync failed` means they did and
   the API call is the problem.

## `cannot determine root domain` / `no zone found`

The zone is looked up by the effective TLD+1 of the hostname — `a.b.example.com`
resolves to zone `example.com` — and that zone must exist **in the account named
by `CLOUDFLARE_ACCOUNT_ID`**. A hostname in a zone owned by another account
fails here, as does a bare hostname with no registrable domain.

## `multiple records found for …`

DNS lookup during removal matches records whose name *contains* the hostname, so
if both `app.example.com` and `staging.app.example.com` exist, deleting the
former is refused rather than risking the wrong record. Creation is
exact-match and idempotent, so this only shows up on the delete path with
`DELETE_DNS_ON_REMOVAL=true`. Delete the record by hand.

## A removed service's hostname still resolves

Expected, in three different ways:

- **Nothing was pending.** Removals are only tracked for services swarmctl saw
  *while running* as tunnel-enabled. If it restarted after the service was
  created, the in-memory hostname cache is empty and the removal is ignored.
  A later removal of any other tunnel-enabled service will still catch the
  orphan, because reconciliation is global.
- **The delay has not elapsed.** Default 30 minutes, counted from the remove
  event.
- **DNS is intentionally left behind.** By default only the ingress entry and
  the Access application are removed. The CNAME survives and resolves to
  Cloudflare's 404 catch-all. Set `DELETE_DNS_ON_REMOVAL=true` to remove it.

## A hostname I manage by hand disappeared

Reconciliation removes every ingress entry in the tunnel that no running
tunnel-enabled service claims — it does not distinguish "someone else put this
here". Use a separate tunnel for manually managed ingress.

## Deploy endpoint returns 500

The body is the Docker error verbatim.

| Body contains | Cause |
|---|---|
| `service <name> not found` | Wrong name. It is `<stack>_<service>`, not the stack or container name. |
| `update out of sequence` | Two updates raced on the same service. Retry. |
| `invalid reference format` | Malformed `image` query value. |
| `permission denied` on the socket | Socket not mounted, or the task is not on a manager node. |

A `401` instead means the header is not exactly `Bearer <token>`; a `429` means
you exceeded 10 requests/minute from that IP.

## Cloudflare syncs fail repeatedly

Each failing sync is retried 5 times (5s, 10s, 20s, 40s) and then abandoned
until the next event for that service. Check `swarmctl_cloudflare_sync_total{status="error"}`
and the error text: `9109` style codes mean the global key/email pair is wrong,
and a 404 on the configuration endpoint usually means `CLOUDFLARE_TUNNEL_ID`
belongs to a different account than `CLOUDFLARE_ACCOUNT_ID`.

## Metrics look wrong

- No `/metrics` line in the HTTP counters: correct, scrapes are excluded on
  purpose.
- Everything under `endpoint="unmatched"`: those are 404s — scanners, or a typo
  in your deploy URL.
- `swarmctl_auth_failures_total` climbing steadily: something is calling `/v1`
  with a stale token. CI, usually.
