package main

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/namespaces"
	"github.com/urfave/cli"
)

var stopCommand = cli.Command{
	Name:  "stop",
	Usage: "stop a running service",
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
		task, err := container.Task(ctx, nil)
		if err != nil {
			return err
		}
		wait, err := task.Wait(ctx)
		if err != nil {
			return err
		}
		if err := container.Update(ctx, withStatus(containerd.Stopped)); err != nil {
			return err
		}
		<-wait
		return nil
	},
}
