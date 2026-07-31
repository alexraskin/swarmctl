---
title: Getting started
weight: 1
---

## What you need first

- A Docker **Swarm** cluster (swarmctl calls manager-only APIs — service list,
  inspect, update — so it must be scheduled on a manager).
- A Cloudflare account with a **named tunnel** already created, and
  `cloudflared` running somewhere in the cluster with that tunnel's token.
  swarmctl edits the tunnel's *ingress configuration*; it does not run the
  tunnel.
- A Cloudflare **scoped API token** with tunnel, Access, DNS, and zone-read
  permissions — see [Configuration](../configuration/#api-token-permissions) for
  the exact four.
- Optionally, one or more [shoutrrr](https://containrrr.dev/shoutrrr/) URLs for
  container-event alerts. Leave `NOTIFICATION_URLS` unset and swarmctl runs fine
  without alerting.

## Deploy it

swarmctl ships as `ghcr.io/alexraskin/swarmctl:latest`. The image listens on
`9000` and healthchecks itself against `/ping`.

```yaml
# docker-compose.swarmctl.yml — deploy with: docker stack deploy -c … swarmctl
services:
  server:
    image: ghcr.io/alexraskin/swarmctl:latest
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      ENVIRONMENT: production
      AUTH_TOKEN: /run/secrets/swarmctl_auth_token
      CLOUDFLARE_API_TOKEN: /run/secrets/cloudflare_api_token
      CLOUDFLARE_ACCOUNT_ID: /run/secrets/cloudflare_account_id
      CLOUDFLARE_TUNNEL_ID: /run/secrets/cloudflare_tunnel_id
      NOTIFICATION_URLS: /run/secrets/notification_urls
      SERVICE_REMOVAL_DELAY_MINUTES: "30"
      DELETE_DNS_ON_REMOVAL: "false"
    secrets:
      - swarmctl_auth_token
      - cloudflare_api_token
      - cloudflare_account_id
      - cloudflare_tunnel_id
      - notification_urls
    deploy:
      placement:
        constraints:
          - node.role == manager
      labels:
        - "cloudflared.tunnel.enabled=true"
        - "cloudflared.tunnel.port=9000"
        - "cloudflared.tunnel.hostname=swarmctl.example.com"

secrets:
  swarmctl_auth_token:
    external: true
  # …and the rest
```

A worked example lives in
[alexraskin/infrastructure](https://github.com/alexraskin/infrastructure/blob/main/swarmctl/docker-compose.swarmctl.yml).

{{< callout type="info" >}}
Any variable whose value starts with `/` and resolves to an existing file is
read from that file and trimmed — that is how the Docker secret paths above
work. Anything else is used literally, so plain env vars are fine for
development. See [Configuration](../configuration/).
{{< /callout >}}

Note that swarmctl labels *itself* for the tunnel in this example. That is the
normal setup: it publishes its own hostname on first sync, and then CI reaches
it there.

## First call

```bash
curl -X POST -H "Authorization: Bearer $AUTH_TOKEN" \
  "https://swarmctl.example.com/v1/update/web_server?image=ghcr.io/you/web:sha-abc123"
```

```json
{ "success": true, "oldVersion": 412, "newVersion": 413 }
```

`web_server` is the **Swarm service name** (`<stack>_<service>`), not the stack
name and not a container name. The endpoint changes only the image in the
service's container spec and re-submits the existing spec, so replica counts,
labels, mounts, and networks are untouched.

Health and version need no token:

```bash
curl https://swarmctl.example.com/ping       # "."
curl https://swarmctl.example.com/version
```

## Give a service a hostname

Add labels to any service in the swarm and redeploy it:

```yaml
deploy:
  labels:
    - "cloudflared.tunnel.enabled=true"
    - "cloudflared.tunnel.port=80"
    - "cloudflared.tunnel.hostname=app.example.com"
```

On the resulting Docker `service update` event swarmctl adds
`app.example.com → http://<serviceName>:80` to the tunnel ingress and creates a
proxied CNAME to `<tunnel-id>.cfargotunnel.com`. Full contract in
[Service labels](../labels/).

{{< callout type="warning" >}}
Labels must be under `deploy.labels` (service labels), not the top-level
`labels:` key, which sets *container* labels. swarmctl reads
`Service.Spec.Labels`, so container labels are invisible to it.
{{< /callout >}}

## Run it locally

```bash
git clone https://github.com/alexraskin/swarmctl && cd swarmctl
mise trust && mise install
cp .env.example .env      # fill in every value
mise run run              # go run . -port 8080 -debug
```

The process needs a reachable Docker socket, and it calls `os.Exit(-1)` at
startup if any required variable is missing. `-debug` turns on the `DEBUG`-level
logs, which is where all the reconciler detail goes.
