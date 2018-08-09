package main

import (
	"github.com/containerd/containerd"
	"github.com/crosbymichael/boss/system"
	"github.com/crosbymichael/boss/systemd"
	"github.com/urfave/cli"
)

var deleteCommand = cli.Command{
	Name:  "delete",
	Usage: "delete a service",
	Action: func(clix *cli.Context) error {
		id := clix.Args().First()
		ctx := system.Context()
		client, err := system.NewClient()
		if err != nil {
			return err
		}
		defer client.Close()
		container, err := client.LoadContainer(ctx, id)
		if err != nil {
			return err
		}
		if err := systemd.Stop(ctx, id); err != nil {
			return err
		}
		if err := systemd.Disable(ctx, id); err != nil {
			return err
		}
		return container.Delete(ctx, containerd.WithSnapshotCleanup)
	},
}
