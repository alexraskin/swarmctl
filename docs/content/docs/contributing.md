---
title: Contributing
weight: 8
---

Issues and pull requests are welcome. This is infrastructure for one specific
cluster, so anything that assumes a different setup is worth pointing out.

## Development

[mise](https://mise.jdx.dev) pins the toolchain and provides the tasks; the
first run in a fresh clone needs `mise trust`.

```bash
mise trust && mise install
mise run check      # gofmt lint + go vet + go test — run before committing
mise run build      # static CGO-free binary -> ./swarmctl
mise run run        # go run . -port 8080 -debug
mise run docker     # container image, same build-args as CI
mise tasks          # list everything
```

Single package or single test:

```bash
go test ./server
go test ./server -run TestExtractHostnames
```

CI (`.github/workflows/ci.yaml` on branches and PRs, `build.yaml` on `main`)
installs the same toolchain with `jdx/mise-action` and calls the same tasks, so
a green `mise run check` locally means a green CI.

## Layout

| Package | Responsibility |
|---|---|
| `main.go` | Wiring: config, clients, syncer, server, signal handling. |
| `server` | HTTP routes and handlers, config loading, the five background workers. |
| `internal/docker` | Thin wrapper over the Docker SDK: inspect, update, list, event stream. |
| `internal/cloudflare` | `API` interface + real client, and the `Syncer` that owns the hostname cache. |
| `internal/metrics` | Prometheus collectors and the HTTP middleware. |
| `internal/middle` | Bearer-token auth middleware. |
| `internal/logger` | slog JSON handler wrapped in a Discord webhook handler. |
| `internal/pushover` | Pushover notification client. |
| `internal/ver` | Build-time version info. |

`cloudflare.API` is an interface precisely so the server and syncer can be
tested without touching Cloudflare — see `internal/cloudflare/sync_test.go` and
`server/service_test.go` for the fakes. New Cloudflare calls go on the
interface, not on the concrete client.

## Invariants worth preserving

{{< callout type="warning" >}}
**Do not add a second subscriber to a Docker event stream.** Each watcher is one
goroutine that re-subscribes *in place* after the stream closes. Older versions
respawned a goroutine per reconnect, which multiplied watchers — and therefore
notifications and sync calls — after every daemon restart.
{{< /callout >}}

- **Every tunnel-config write is read-modify-write under the client's mutex.**
  The API replaces the whole ingress document. Adding a write path that skips
  the lock will silently drop other hostnames.
- **The `http_status:404` catch-all must stay last.** Cloudflare rejects a
  config where it is not the final rule.
- **Keep Cloudflare calls off the event loop.** Sync runs in its own goroutine
  with bounded retries; blocking the watcher stalls every other service event.
- **Auth and the rate limiter belong to the `/v1` group only.** Moving them up
  to the router breaks Prometheus scraping and poisons the auth-failure metric.
- **Watch label cardinality.** Prometheus labels must come from route patterns
  and bounded sets, never from raw request paths.
- **Removal is convergence, not bookkeeping.** The reconciler diffs live tunnel
  config against running services. Resist adding per-service "what did this own"
  state; the diff is what makes a restart harmless.

New behaviour should come with a test. The existing tests cover the syncer
(including Access apps and cache invalidation), the auth middleware, config
loading, and hostname extraction. The reconciler itself is not covered yet — a
good place to start.

## These docs

Hugo + [Hextra](https://imfing.github.io/hextra/), sources under `docs/content`:

```bash
mise run docs-dev     # http://localhost:1313/
mise run docs-build   # -> docs/public
```

Pushes to `main` that touch `docs/**` deploy to GitHub Pages automatically
(`.github/workflows/docs.yml`). Every page has an "Edit this page" link at the
bottom.
