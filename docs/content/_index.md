---
title: swarmctl
layout: hextra-home
---

{{< hextra/hero-badge >}}
  <div class="hx:w-2 hx:h-2 hx:rounded-full hx:bg-primary-400"></div>
  <span>Single Go binary · runs inside the swarm</span>
{{< /hextra/hero-badge >}}

<div class="hx:mt-6 hx:mb-6">
{{< hextra/hero-headline >}}
  Deploy hook and Cloudflare&nbsp;<br class="hx:sm:block hx:hidden" />reconciler for Docker Swarm
{{< /hextra/hero-headline >}}
</div>

<div class="hx:mb-12">
{{< hextra/hero-subtitle >}}
  swarmctl gives CI one endpoint to roll a new image into a running service,&nbsp;<br class="hx:sm:block hx:hidden" />
  and keeps Cloudflare Tunnel ingress, DNS, and Access apps in sync with the labels on your services.
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx:mb-6">
{{< hextra/hero-button text="Get started" link="docs/getting-started/" >}}
</div>

<div class="hx:mt-6"></div>

{{< hextra/feature-grid >}}
  {{< hextra/feature-card
    title="One POST deploys a service"
    subtitle="POST /v1/update/{service}?image=… swaps the image on a running Swarm service and returns the old and new spec versions. Nothing else in the spec is touched."
  >}}
  {{< hextra/feature-card
    title="Ingress from labels"
    subtitle="A service that carries cloudflared.tunnel.* labels gets a tunnel ingress entry and a proxied CNAME. No per-service config lives in swarmctl."
  >}}
  {{< hextra/feature-card
    title="Cleans up after itself"
    subtitle="Removing a service parks a pending removal; after a delay the reconciler diffs the live tunnel config against running services and drops the orphans."
  >}}
  {{< hextra/feature-card
    title="SSO with one more label"
    subtitle="cloudflared.tunnel.access.policy=<id> puts a self-hosted Cloudflare Access application in front of every hostname, attached to a reusable policy."
  >}}
  {{< hextra/feature-card
    title="Tells you when things die"
    subtitle="Container die/restart/crash events become Pushover pushes, deduped per container so a crash loop is one notification a minute, not fifty."
  >}}
  {{< hextra/feature-card
    title="Prometheus and Discord"
    subtitle="A /metrics endpoint outside auth for scraping, a Grafana dashboard in the repo, and WARN+ logs mirrored to a Discord webhook."
  >}}
{{< /hextra/feature-grid >}}

<div class="hx:mt-12"></div>

```bash
# deploy a new image from CI
curl -X POST -H "Authorization: Bearer $AUTH_TOKEN" \
  "https://swarmctl.example.com/v1/update/web_server?image=ghcr.io/you/web:sha-abc123"
```

```yaml
# and this is all a service needs to get a hostname
labels:
  - "cloudflared.tunnel.enabled=true"
  - "cloudflared.tunnel.port=80"
  - "cloudflared.tunnel.hostname=app.example.com"
```
