# swarmctl

![swarmctl](https://i.gyazo.com/41976189f3f92b5ccdacc914b4b73e37.png)

Docker Swarm tool:
- RESTful API for service updates
- Automatic service discovery via Docker Tags
- Cloudflare Tunnel integration with DNS management

For an example of how to deploy the swarmctl server to the swarm cluster, see the [docker-compose.swarmctl.yml](https://github.com/alexraskin/infrastructure/blob/main/swarmctl/docker-compose.swarmctl.yml) file.


After the swarmctl server is deployed, you can use it to update the services in the swarm cluster.

```
curl -X POST -H "Authorization: your-token" https://swarmctl.your-domain.com/v1/update/your-service?image=your-image
```
