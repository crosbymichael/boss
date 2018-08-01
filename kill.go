package main

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/namespaces"
	"github.com/urfave/cli"
	"golang.org/x/sys/unix"
)

var killCommand = cli.Command{
	Name:  "kill",
	Usage: "kill a running service",
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

		if err := register.EnableMaintainance(id, "manual kill"); err != nil {
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
