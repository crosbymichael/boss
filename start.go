package main

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/runtime/restart"
	"github.com/urfave/cli"
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
