---
title: Documentation
weight: 1
next: /docs/getting-started
---

swarmctl is a small Go daemon that runs **inside** a Docker Swarm cluster, on a
manager node, with the Docker socket mounted. It does two largely independent
jobs:

1. **A deploy endpoint.** `POST /v1/update/{serviceName}?image=…` pulls a new
   image into a running Swarm service. CI calls it instead of holding a SSH key.
2. **A background reconciler.** It watches the Docker event stream and keeps
   Cloudflare Tunnel ingress, DNS records, and Access applications in sync with
   the services that carry `cloudflared.tunnel.*` labels.

Service discovery is entirely label-driven — swarmctl itself has no per-service
configuration.

## Start here

{{< cards >}}
  {{< card link="getting-started/" title="Getting started" subtitle="Deploy swarmctl to the swarm, give it credentials, make the first call." >}}
  {{< card link="labels/" title="Service labels" subtitle="The label contract: enabled, port, hostname, access policy." >}}
  {{< card link="configuration/" title="Configuration" subtitle="Every environment variable, and how Docker secrets are read." >}}
  {{< card link="api/" title="HTTP API" subtitle="Endpoints, auth, rate limiting, response shapes." >}}
{{< /cards >}}

## Going deeper

{{< cards >}}
  {{< card link="how-it-works/" title="How it works" subtitle="Event watchers, the sync path, and removal reconciliation." >}}
  {{< card link="observability/" title="Observability" subtitle="Prometheus metrics, the Grafana dashboard, Discord logging, Pushover alerts." >}}
  {{< card link="troubleshooting/" title="Troubleshooting" subtitle="Startup exits, missing zones, duplicate DNS records, hostnames that never disappear." >}}
  {{< card link="contributing/" title="Contributing" subtitle="Dev loop, package layout, and the invariants to preserve." >}}
{{< /cards >}}

## The one thing to know first

{{< callout type="warning" >}}
swarmctl talks to the Docker API as a **manager**, and the deploy endpoint will
change the image of any service you name. The bearer token in `AUTH_TOKEN` is
therefore root-equivalent on the cluster. Only `/v1/*` is authenticated —
`/ping`, `/version`, and `/metrics` are deliberately public, so put the service
behind the tunnel and treat the token as a cluster credential.
{{< /callout >}}
