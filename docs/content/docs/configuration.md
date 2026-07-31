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
| `CLOUDFLARE_API_TOKEN` | Scoped Cloudflare API token. Permissions below. |
| `CLOUDFLARE_ACCOUNT_ID` | Account that owns the tunnel, the zones, and the Access apps. |
| `CLOUDFLARE_TUNNEL_ID` | The named tunnel whose ingress config swarmctl edits. Also the CNAME target: `<id>.cfargotunnel.com`. |
| `ENVIRONMENT` | Free-form environment name, attached to every log line as `env`. |

### API token permissions

The client authenticates with `Authorization: Bearer`, so `CLOUDFLARE_API_TOKEN`
is a **scoped API token** — create one at *My Profile → API Tokens → Create
Token → Custom token*. Four permissions, matching exactly what swarmctl calls:

| Scope | Permission | Used by |
|---|---|---|
| Account | Cloudflare Tunnel : Edit | reading and rewriting the tunnel's ingress config |
| Account | Access: Apps and Policies : Edit | `cloudflared.tunnel.access.policy` — creating and deleting Access apps |
| Zone | DNS : Edit | creating and deleting the proxied CNAMEs |
| Zone | Zone : Read | resolving a hostname to its zone ID |

Restrict account resources to the account in `CLOUDFLARE_ACCOUNT_ID`, and zone
resources to the zones you actually publish hostnames in. A token scoped this
way cannot touch billing, members, or other accounts — the reason to prefer it
over a global key, which cannot be narrowed at all.

{{< callout type="warning" >}}
`AUTH_TOKEN` is still root-equivalent on the *cluster* — the deploy endpoint can
change the image of any service. Scoping the Cloudflare token does not change
that; keep both in Docker secrets.
{{< /callout >}}

## Optional

| Variable | Default | Effect |
|---|---|---|
| `NOTIFICATION_URLS` | *(none)* | Comma- or newline-separated [shoutrrr](https://containrrr.dev/shoutrrr/) URLs that receive container-event alerts. Unset means alerts are off — swarmctl logs a warning at startup and starts normally. |
| `SERVICE_REMOVAL_DELAY_MINUTES` | `30` | How long a removed service stays "pending" before the reconciler runs. Covers rolling redeploys that remove and recreate a service. Unparseable values fall back to the default. |
| `DELETE_DNS_ON_REMOVAL` | `false` | When `true` (case-insensitive), reconciliation also deletes the hostname's CNAME record, not just the ingress entry and Access app. |

### Notification URLs

Every [shoutrrr service](https://containrrr.dev/shoutrrr/v0.8/services/overview/)
works — Discord, Slack, Telegram, ntfy, Pushover, Gotify, Matrix, email, or a
generic webhook. List more than one and each alert goes to all of them.

```bash
NOTIFICATION_URLS=discord://token@id,pushover://shoutrrr:apiToken@userKey
```

A malformed or unknown-scheme URL is rejected **at startup**, not at the first
alert, so a typo fails loudly instead of silently swallowing pushes. Delivery is
concurrent and one dead destination does not block the others.

Because these URLs contain credentials, point `NOTIFICATION_URLS` at a Docker
secret file rather than inlining it; a secret file may hold one URL per line.

## Docker secrets

Both `getSecretOrEnv` (required vars) and `getOptionalSecretOrEnv`
(`NOTIFICATION_URLS`) treat a value that **starts with `/` and exists as a
file** as a path, read it, and trim surrounding whitespace. Otherwise the value
is used literally. The difference is what happens when the value is empty: the
required form exits, the optional form returns nothing and carries on.

```yaml
environment:
  AUTH_TOKEN: /run/secrets/swarmctl_auth_token   # read from the file
  ENVIRONMENT: production                        # used as-is
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
