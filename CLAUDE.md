# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`swarmctl` is a Go daemon that runs **inside** a Docker Swarm cluster. It does two largely independent jobs:

1. **REST API** — `POST /v1/update/{serviceName}?image=...` pulls a new image into a running Swarm service (used by CI to deploy). See `.github/workflows/build.yaml` — the pipeline builds the image, then calls swarmctl's own endpoint to deploy itself.
2. **Background reconciler** — watches the Docker event stream and keeps Cloudflare Tunnel ingress + DNS in sync with services that carry `cloudflared.tunnel.*` labels. Service discovery is label-driven; nothing is configured per-service in swarmctl itself.

## Commands

```bash
go build ./...                              # build
go run . -port 8080 -debug                  # run locally (needs Docker socket + .env)
go vet ./...                                # vet
go test ./...                               # tests (note: no test files exist yet)
go test ./server -run TestName              # single test, once tests exist
docker build -t swarmctl .                  # multi-arch build (matches CI)
```

Requires a `.env` file (copy `.env.example`) and access to the Docker socket. The binary refuses to start if any required env var is unset (`getSecretOrEnv` calls `os.Exit(-1)`).

## Architecture

`main.go` wires everything: loads config, builds the Docker / Cloudflare / Pushover clients + the Cloudflare `Syncer`, constructs the `Server`, and runs it. `Server.Start()` (`server/server.go`) launches the HTTP listener **and three background goroutines** — these run for the life of the process:

- `startDockerMonitor` (`server/service.go`) — fans out to three Docker event watchers:
  - `monitorServiceEvents`: on service create/update, if labeled `cloudflared.tunnel.enabled=true`, caches hostnames in `serviceHostnames` and calls `cfSyncer.SyncService`.
  - `dockerEventsMonitor`: on container die/restart/crash, sends a Pushover notification. Deduped via `recentEvents` sync.Map with a 1-minute cooldown.
  - `monitorServiceRemovals`: on service remove, records a `pendingRemoval` (only for previously-tunnel-enabled services).
- `startEventCleanup` — periodically evicts stale entries from `recentEvents`.
- `startRemovalProcessor` — every minute, promotes `pendingRemovals` older than `ServiceRemovalDelayMinutes` into a `reconcileTunnelConfig()` pass that diffs running services against the live tunnel config and removes orphaned ingress entries (and optionally DNS records, if `DELETE_DNS_ON_REMOVAL=true`).

Each event watcher self-reconnects by relaunching its own goroutine when the Docker error channel closes — be careful not to introduce duplicate watchers when editing them.

### Key boundaries

- `internal/docker` — thin wrapper over the Docker SDK. `UpdateDockerService` mutates only `ContainerSpec.Image` and re-submits the existing spec/version.
- `internal/cloudflare` — `API` is an interface (`sync.go` / `cloudflare.go`); `Server.cfClient` holds the interface so it can be mocked. `Syncer` owns an in-memory `cache` of hostname→ingress, lazily loaded on first sync.
- `internal/metrics` — Prometheus collectors (`promauto`) + middleware; exposed at `/metrics`. Helper funcs (`RecordDockerServiceUpdate`, etc.) are called throughout, not the collectors directly.
- `internal/middle` — bearer-token auth. Expects `Authorization: Bearer <token>`; compares `token[7:]` to `AUTH_TOKEN` with `subtle.ConstantTimeCompare`.
- `internal/logger` — slog over a Discord webhook handler: JSON to stdout at the configured level, plus Discord delivery for `WARN`+.
- `internal/pushover`, `internal/ver` — Pushover client; build-info version (populated from VCS build settings).

### Label contract (service discovery)

Services opt in with Docker labels — both `SyncService` and `extractHostnames` read these:

- `cloudflared.tunnel.enabled=true` — required to be considered.
- `cloudflared.tunnel.port=<port>` — tunnel target becomes `http://<serviceName>:<port>`.
- `cloudflared.tunnel.hostname=host` — primary hostname; comma-separated for multiple.
- `cloudflared.tunnel.N.hostname=host` — any label ending in `.hostname` adds more hostnames.
- `cloudflared.tunnel.access.policy=<policyID>` — optional. If set, swarmctl ensures a self-hosted Cloudflare Access (SSO) application protects each hostname, attached to the given reusable Access policy ID. The Access app is removed during removal reconciliation. Idempotent (`EnsureAccessApp`/`DeleteAccessApp` in `internal/cloudflare/cloudflare.go`).

## HTTP surface (`server/routes.go`)

chi router. Global middleware: RequestID, RealIP, Logger, Recoverer, `/ping` heartbeat, **auth (applies to everything)**, metrics, and a 10-req/min rate limit. Routes: `GET /version`, `GET /metrics`, `POST /v1/update/{serviceName}`.

## Config (`server/config.go`)

All config comes from env vars via `getSecretOrEnv`: if a value starts with `/` and the path exists, it's read as a **Docker secret file** (trimmed); otherwise treated as a literal. Note the env var is spelled `ENVIROMENT` (sic). Numeric/bool extras: `SERVICE_REMOVAL_DELAY_MINUTES` (default 30), `DELETE_DNS_ON_REMOVAL` (default false).

## Deploy

Push to `main` triggers `.github/workflows/build.yaml`: build + push to `ghcr.io/alexraskin/swarmctl`, then deploy by POSTing to the live swarmctl `/v1/update` endpoint, then notify via shoutrrr. Container listens on `9000` in the image (`CMD ["-port", "9000"]`); healthcheck hits `/ping`.
