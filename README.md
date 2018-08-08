# boss

![ross](http://gifs.joelglovier.com/boss/like-a-ross.gif)

This is my tool, built for me, to run containers on my own infra.
Posting the code publicly if others can find inspiration from it and to see how they can use containerd to build the container platform that they want.
It's single node right now, no schedulers.
You manage it on the node.

## Ideas

* you should be able to update container resources without restarting a container
* you should be able to update the image without creating a new container
* you should be able to rollback to a previous container state
* containers should be able to migrate across nodes, live or otherwise, and keep all their data
* services are automatically registered and found via DNS
* don't bother me with fancy graphs and metrics, just alert me when something's wrong
* logs on disk suck, apps should send to things like sentry when they can, else go to system logger
* KISS

## Bits and pieces

* runtime: `containerd`
* cli: `boss`

I use `macvlan` so all containers have private IPs on my network.
This makes DNS and consul a good fit.

That's about it.

Configuration is handled via toml:

```toml
id = "timescale"
image = "docker.io/timescale/timescaledb:latest-pg10"
env = [
	"POSTGRES_PASSWORD=somethings",
]
network = "cni"

[[mounts]]
	type = "bind"
	source = "/containers/volumes/timescale"
	destination = "/var/lib/postgresql/data"
	options = ["rbind", "rw"]

[resources]
	memory = 24000
	cpu = 7.0

[gpus]
	devices = [0]
	capbilities = ["utility"]

[services]
	[services.postgres]
		port = 5432
		labels = ["prod"]
```
