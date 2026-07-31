---
title: Service labels
weight: 2
---

Everything swarmctl knows about a service comes from its **service labels**
(`Service.Spec.Labels`, i.e. `deploy.labels:` in a compose file). Nothing is
configured per-service inside swarmctl.

## The contract

| Label | Required | Meaning |
|---|---|---|
| `cloudflared.tunnel.enabled` | yes | Must be exactly `true`. Anything else and the service is ignored entirely. |
| `cloudflared.tunnel.port` | yes in practice | Port on the service. The ingress target becomes `http://<serviceName>:<port>`. |
| `cloudflared.tunnel.hostname` | yes | Primary hostname. Comma-separated for several. |
| `cloudflared.tunnel.<n>.hostname` | no | Any label **ending in** `.hostname` adds more hostnames. The `<n>` is arbitrary. |
| `cloudflared.tunnel.access.policy` | no | ID of a reusable Cloudflare Access policy. Puts SSO in front of every hostname of this service. |

The service name in the target URL is the Swarm service name, resolved over the
overlay network by Docker's internal DNS. The tunnel origin is always plain
`http://` — TLS terminates at Cloudflare.

## One hostname

```yaml
deploy:
  labels:
    - "cloudflared.tunnel.enabled=true"
    - "cloudflared.tunnel.port=80"
    - "cloudflared.tunnel.hostname=app.example.com"
```

## Several hostnames

Two equivalent forms — indexed labels, or one comma-separated value:

```yaml
deploy:
  labels:
    - "cloudflared.tunnel.enabled=true"
    - "cloudflared.tunnel.port=80"
    - "cloudflared.tunnel.0.hostname=example.com"
    - "cloudflared.tunnel.1.hostname=www.example.com"
```

```yaml
deploy:
  labels:
    - "cloudflared.tunnel.enabled=true"
    - "cloudflared.tunnel.port=80"
    - "cloudflared.tunnel.hostname=example.com,www.example.com"
```

Every hostname gets its own ingress entry and its own DNS record, all pointing
at the same `http://<serviceName>:<port>` target. There is no per-hostname port.

{{< callout type="info" >}}
The suffix match is literally `strings.HasSuffix(key, ".hostname")`. A label
like `myapp.hostname=…` on a tunnel-enabled service therefore *also* becomes a
tunnel hostname, whether or not you meant it to. Keep unrelated labels away from
that suffix.
{{< /callout >}}

## Access (SSO)

Add the ID of an existing [reusable Access
policy](https://developers.cloudflare.com/cloudflare-one/policies/access/) and
swarmctl creates a self-hosted Access application per hostname, attached to that
policy:

```yaml
deploy:
  labels:
    - "cloudflared.tunnel.enabled=true"
    - "cloudflared.tunnel.port=80"
    - "cloudflared.tunnel.hostname=private.example.com"
    - "cloudflared.tunnel.access.policy=1f2e3d4c-…"
```

List your policy IDs:

```bash
curl -s "https://api.cloudflare.com/client/v4/accounts/$CLOUDFLARE_ACCOUNT_ID/access/policies" \
  -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
  | jq '.result[] | {id, name, decision}'
```

The application is matched by **exact domain**, created only when missing, and
deleted during removal reconciliation. Re-syncing an already-protected hostname
is a no-op.

{{< callout type="warning" >}}
Removing the `access.policy` label does **not** remove the Access application —
`EnsureAccessApp` only ever creates. Only removing the whole service (or the
hostname) triggers `DeleteAccessApp`. Delete the app in the Cloudflare dashboard
if you drop the label from a service that stays up.
{{< /callout >}}

## What happens when labels change

| Change | Effect |
|---|---|
| Add a hostname | New ingress entry + new CNAME on the next service update event. |
| Change the port | Target URL differs from the cached one, so the ingress entry is rewritten. DNS is untouched (it points at the tunnel, not the port). |
| Remove one hostname, service stays up | Nothing immediate. The stale ingress entry survives until a *removal* of some tunnel-enabled service triggers reconciliation, which drops any hostname no running service claims. |
| Set `enabled=false` or drop the labels | Same as above — cleanup only happens on the removal-driven reconcile pass. |
| Remove the service | Pending removal recorded, reconciled after `SERVICE_REMOVAL_DELAY_MINUTES`. |

See [How it works](../how-it-works/) for the reconciliation pass in detail.
