# boss

![ross](http://gifs.joelglovier.com/boss/like-a-ross.gif)

**Disclaimer:**

Posting the code publicly if others can find inspiration from it and to see how they can use containerd to build the container platform that they want.
It's single node right now, no schedulers, you manage it on the node.

This code is open source and it should work for most setups on modern systems.
If you don't have a modern system, then you are holding us all back and you need to upgrade.
If you use a distro that lives in the past, maybe you should switch.

This project is built for me, for my servers, running the way I think infrastructure should run.
It's very opinionated.
I'll merge PRs when they make sense for the project, but if I don't merge your PR, don't take it personal.
I need a place to try out ideas and only be responsible to myself, I write enough code that is used by many
and feel the responsibility of my actions and code every day.
This is my safe space where I only answer to myself.

Feel free to fork this project and make it something great for your own needs, I encourage it.
Take the code, try out crazy ideas, experiment, and share your creations with others.

## CLI

```
NAME:
   boss - run containers like a ross

USAGE:
   boss [global options] command [command options] [arguments...]

VERSION:
   8

DESCRIPTION:


                    ___           ___           ___
     _____         /\  \         /\__\         /\__\
    /::\  \       /::\  \       /:/ _/_       /:/ _/_
   /:/\:\  \     /:/\:\  \     /:/ /\  \     /:/ /\  \
  /:/ /::\__\   /:/  \:\  \   /:/ /::\  \   /:/ /::\  \
 /:/_/:/\:|__| /:/__/ \:\__\ /:/_/:/\:\__\ /:/_/:/\:\__\
 \:\/:/ /:/  / \:\  \ /:/  / \:\/:/ /:/  / \:\/:/ /:/  /
  \::/_/:/  /   \:\  /:/  /   \::/ /:/  /   \::/ /:/  /
   \:\/:/  /     \:\/:/  /     \/_/:/  /     \/_/:/  /
    \::/  /       \::/  /        /:/  /        /:/  /
     \/__/         \/__/         \/__/         \/__/

run containers like a boss

COMMANDS:
     build     build
     create    create a container
     delete    delete a service
     init      init boss on a system
     kill      kill a running service
     list      list containers managed via boss
     rollback  rollback a container to a previous revision
     start     start an existing service
     stop      stop a running service
     upgrade   upgrade a container's image but keep its data, like it should be
     help, h   Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --help, -h     show help
   --version, -v  print the version
```

## Ideas

* you should be able to update container resources without restarting a container
* you should be able to update the image without creating a new container
* you should be able to rollback to a previous container state
* containers should be able to migrate across nodes, live or otherwise, and keep all their data
* services are automatically registered and found via DNS
* don't bother me with fancy graphs and metrics, just alert me when something's wrong
* logs on disk suck, apps should send to things like sentry when they can, else go to system logger
* KISS

## Dependencies

You need to have a new containerd version running.
containerd 1.2+.
As containerd 1.2 isn't out yet, use master, like me.

Also a modern systemd based system.
Ubuntu 18.04 server works amazing.
Boss needs to run as root.

### System Configuration

To bootstrap your system write a system config to `/etc/boss/boss.toml` and run init your system with `boss`.

`> boss init`

This will install everything in your system config and get your system up and running.
If you have `consul` configured, it will install, setup, configure DNS, and register other services automatically.

If you have `cni` configured, it will download the `cni` plugins and automatically use it for containers that have `network = "cni"` specified.
You don't have to write a `/etc/cni/net.d` conf or install plugins, `boss` does it all for you.

If you want to be able to run `boss build` on a machine, you add the `buildkit` configuration and `boss`
will get you `buildkitd` up and running and ready to build images.

There is other configurations that I use that are here.
They may not be useful for other people, but again, this is my project built for me and my needs.
**I do what I want in it ;)**

If you hate `boss` and think it's ugly, just run `> boss init --undo` and it will clean itself up and get out of your hair. EZ.

```toml
id = "hostname-01"
iface = "eth0"
domain = "my-domain"

[consul]
        image = "docker.io/crosbymichael/consul:latest"
        bootstrap = true

[buildkit]
        image = "docker.io/crosbymichael/buildkit:latest"

[cni]
        image = "docker.io/crosbymichael/cni:latest"
        type = "macvlan"
        [cni.ipam]
                type = "dhcp"

[nodemetrics]
        image = "docker.io/crosbymichael/nodeexporter:latest"
```

### Container Configuration

To run a container, bootstrap your system then create a `toml` file and run it with `boss`.

`> boss create redis.toml`

```toml
id = "redis"
image = "docker.io/library/redis:3.2-stretch"
network = "cni"

[resources]
        memory = 1024
        cpu = 2.0

[services]
        [services.redis]
                port = 6379
                labels = ["dev"]
```

## License

```
Copyright (c) 2018 Michael Crosby crosbymichael@gmail.com

Permission is hereby granted, free of charge, to any person
obtaining a copy of this software and associated documentation
files (the "Software"), to deal in the Software without
restriction, including without limitation the rights to use, copy,
modify, merge, publish, distribute, sublicense, and/or sell copies
of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be
included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED,
INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT
HOLDERS BE LIABLE FOR ANY CLAIM,
DAMAGES OR OTHER LIABILITY,
WHETHER IN AN ACTION OF CONTRACT,
TORT OR OTHERWISE,
ARISING FROM, OUT OF OR IN CONNECTION WITH
THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
```
