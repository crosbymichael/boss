package main

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/runtime/restart"
	"github.com/hashicorp/consul/api"
	"github.com/urfave/cli"
	"golang.org/x/sys/unix"
)

var startCommand = cli.Command{
	Name:  "start",
	Usage: "start an existing service",
	Action: func(clix *cli.Context) error {
		ctx := namespaces.WithNamespace(context.Background(), clix.GlobalString("namespace"))
		client, err := containerd.New(
			defaults.DefaultAddress,
			containerd.WithDefaultRuntime("io.containerd.runc.v1"),
		)
		if err != nil {
			return err
		}
		defer client.Close()
		id := clix.Args().First()
		container, err := client.LoadContainer(ctx, id)
		if err != nil {
			return err
		}
		return container.Update(ctx, restart.WithStatus(containerd.Running))
	},
}
var killCommand = cli.Command{
	Name:  "kill",
	Usage: "kill a running service",
	Action: func(clix *cli.Context) error {
		consul, err := api.NewClient(api.DefaultConfig())
		if err != nil {
			return err
		}
		ctx := namespaces.WithNamespace(context.Background(), clix.GlobalString("namespace"))
		client, err := containerd.New(
			defaults.DefaultAddress,
			containerd.WithDefaultRuntime("io.containerd.runc.v1"),
		)
		if err != nil {
			return err
		}
		defer client.Close()
		id := clix.Args().First()

		if err := consul.Agent().EnableServiceMaintenance(id, "manual kill"); err != nil {
			return err
		}
		container, err := client.LoadContainer(ctx, id)
		if err != nil {
			return err
		}
		task, err := container.Task(ctx, nil)
		if err != nil {
			return err
		}
		wait, err := task.Wait(ctx)
		if err != nil {
			return err
		}
		if err := task.Kill(ctx, unix.SIGTERM); err != nil {
			return err
		}
		<-wait

		if _, err := task.Delete(ctx); err != nil {
			return err
		}
		return nil
	},
}

var stopCommand = cli.Command{
	Name:  "stop",
	Usage: "stop a running service",
	Action: func(clix *cli.Context) error {
		consul, err := api.NewClient(api.DefaultConfig())
		if err != nil {
			return err
		}
		ctx := namespaces.WithNamespace(context.Background(), clix.GlobalString("namespace"))
		client, err := containerd.New(
			defaults.DefaultAddress,
			containerd.WithDefaultRuntime("io.containerd.runc.v1"),
		)
		if err != nil {
			return err
		}
		defer client.Close()
		id := clix.Args().First()

		if err := consul.Agent().EnableServiceMaintenance(id, "manual stop"); err != nil {
			return err
		}
		container, err := client.LoadContainer(ctx, id)
		if err != nil {
			return err
		}
		task, err := container.Task(ctx, nil)
		if err != nil {
			return err
		}
		wait, err := task.Wait(ctx)
		if err != nil {
			return err
		}
		if err := container.Update(ctx, restart.WithStatus(containerd.Stopped)); err != nil {
			return err
		}
		<-wait
		if _, err := task.Delete(ctx); err != nil {
			return err
		}
		return nil
	},
}

var deleteCommand = cli.Command{
	Name:  "delete",
	Usage: "delete a service",
	Action: func(clix *cli.Context) error {
		ctx := namespaces.WithNamespace(context.Background(), clix.GlobalString("namespace"))
		client, err := containerd.New(
			defaults.DefaultAddress,
			containerd.WithDefaultRuntime("io.containerd.runc.v1"),
		)
		if err != nil {
			return err
		}
		defer client.Close()
		id := clix.Args().First()
		consul, err := api.NewClient(api.DefaultConfig())
		if err != nil {
			return err
		}
		if err := consul.Agent().ServiceDeregister(id); err != nil {
			return err
		}
		container, err := client.LoadContainer(ctx, id)
		if err != nil {
			return err
		}

		return container.Delete(ctx, containerd.WithSnapshotCleanup)
	},
}