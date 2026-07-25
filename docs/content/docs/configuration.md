---
title: Configuration
weight: 3
---

All configuration is environment variables, read once at startup by
`server.LoadConfig`. A `.env` file in the working directory is loaded first if
present (missing `.env` is logged, not fatal).

## Required

Every one of these must be non-empty. If any is missing, swarmctl logs
`Environment variable is not set` and exits with status `-1` before the listener
starts.

| Variable | What it is |
|---|---|
| `AUTH_TOKEN` | Bearer token for `/v1/*`. Compared in constant time. Root-equivalent on the cluster — treat it that way. |
| `CLOUDFLARE_API_KEY` | Cloudflare **global API key**. |
| `CLOUDFLARE_API_EMAIL` | Account email that goes with the global key. |
| `CLOUDFLARE_ACCOUNT_ID` | Account that owns the tunnel, the zones, and the Access apps. |
| `CLOUDFLARE_TUNNEL_ID` | The named tunnel whose ingress config swarmctl edits. Also the CNAME target: `<id>.cfargotunnel.com`. |
| `PUSHOVER_API_KEY` | Pushover *application* key for container-event pushes. |
| `PUSHOVER_RECIPIENT` | Pushover user/group key that receives them. |
| `WEBHOOK_URL` | Discord webhook. `WARN` and above are mirrored to it. |
| `ENVIROMENT` | Free-form environment name, attached to every log line as `env`. Note the spelling — it is misspelled in the code and the variable name must match. |

{{< callout type="warning" >}}
The Cloudflare client authenticates with `X-Auth-Email` + `X-Auth-Key`, so this
is the **global API key**, not a scoped API token. A global key is
account-wide and cannot be narrowed; keep it in a Docker secret and rotate it if
it ever lands in a shell history.
{{< /callout >}}

## Optional

| Variable | Default | Effect |
|---|---|---|
| `SERVICE_REMOVAL_DELAY_MINUTES` | `30` | How long a removed service stays "pending" before the reconciler runs. Covers rolling redeploys that remove and recreate a service. Unparseable values fall back to the default. |
| `DELETE_DNS_ON_REMOVAL` | `false` | When `true` (case-insensitive), reconciliation also deletes the hostname's CNAME record, not just the ingress entry and Access app. |

## Docker secrets

`getSecretOrEnv` treats a value that **starts with `/` and exists as a file** as
a path, reads it, and trims surrounding whitespace. Otherwise the value is used
literally.

```yaml
environment:
  AUTH_TOKEN: /run/secrets/swarmctl_auth_token   # read from the file
  CLOUDFLARE_API_EMAIL: you@example.com          # used as-is
secrets:
  - swarmctl_auth_token
```

A `/`-prefixed value whose file does *not* exist falls through and is used as a
literal string, which will fail later at the API rather than at startup.

## Flags

```
-port string   port to listen on (default "8080")
-debug         enable debug logging
```

The container image sets `CMD ["-port", "9000"]`, so the published image listens
on `9000`. Almost all reconciler detail — sync results, event dedup, removal
bookkeeping — logs at `DEBUG`; at the default level you mostly see errors and
completed service updates.

## Shutdown

`SIGINT` / `SIGTERM` start a 5-second graceful shutdown: the HTTP server stops
accepting, the shared context is cancelled, and all five background workers are
waited on. If shutdown does not finish in time the server is closed hard.
