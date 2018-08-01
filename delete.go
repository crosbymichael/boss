package main

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/namespaces"
	"github.com/urfave/cli"
)

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
		if err := register.Deregister(id); err != nil {
			return err
		}
		container, err := client.LoadContainer(ctx, id)
		if err != nil {
			return err
		}

		return container.Delete(ctx, containerd.WithSnapshotCleanup)
	},
}
