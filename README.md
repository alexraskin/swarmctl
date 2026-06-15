# swarmctl

![swarmctl](https://i.gyazo.com/41976189f3f92b5ccdacc914b4b73e37.png)

Docker Swarm tool:
- REST API for service updates
- Automatic service discovery via Docker Tags
- Cloudflare Tunnel integration with DNS management
- Pushover notifications for service updates
- Prometheus metrics
- Logging to Discord

1. Copy the `.env.example` file to `.env` and fill in the values.
2. Deploy the swarmctl server to the swarm cluster. Example: [docker-compose.swarmctl.yml](https://github.com/alexraskin/infrastructure/blob/main/swarmctl/docker-compose.swarmctl.yml) file.

After the swarmctl server is deployed, you can use it to update the services in the swarm cluster.

```bash
curl -X POST -H "Authorization: Bearer your-token" https://swarmctl.your-domain.com/v1/update/your-service?image=your-image
```
3. Add docker labels to the services you want to update. Example:

```yaml
labels:
    - "cloudflared.tunnel.enabled=true"
    - "cloudflared.tunnel.port=80"
    - "cloudflared.tunnel.hostname=your-domain.com"
```
It also supports multiple hostnames. Example:

```yaml
labels:
    - "cloudflared.tunnel.enabled=true"
    - "cloudflared.tunnel.port=80"
    - "cloudflared.tunnel.0.hostname=your-domain.com"
    - "cloudflared.tunnel.1.hostname=2.your-domain.com"
```

### Protecting a service behind Cloudflare Access (SSO)

Add the `cloudflared.tunnel.access.policy` label with the ID of an existing
[reusable Access policy](https://developers.cloudflare.com/cloudflare-one/policies/access/). swarmctl
will create a self-hosted Access application for each hostname, attach that
policy, and remove the application when the service is removed.

```yaml
labels:
    - "cloudflared.tunnel.enabled=true"
    - "cloudflared.tunnel.port=80"
    - "cloudflared.tunnel.hostname=your-domain.com"
    - "cloudflared.tunnel.access.policy=<reusable-access-policy-id>"
```

List your reusable policy IDs with:

```bash
curl -s "https://api.cloudflare.com/client/v4/accounts/$CLOUDFLARE_ACCOUNT_ID/access/policies" \
  -H "X-Auth-Email: $CLOUDFLARE_API_EMAIL" -H "X-Auth-Key: $CLOUDFLARE_API_KEY" \
  | jq '.result[] | {id, name, decision}'
```

4. Deploy the services to the swarm cluster.

```bash
docker stack deploy -c docker-compose.yml your-stack
```

5. If you want to use the Prometheus metrics, you can use the following URL:

```
http://swarmctl.your-domain.com/metrics
```
