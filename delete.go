package main

import (
	"github.com/containerd/containerd"
	"github.com/crosbymichael/boss/systemd"
	"github.com/urfave/cli"
)

var deleteCommand = cli.Command{
	Name:   "delete",
	Usage:  "delete a service",
	Before: ReadyBefore,
	Action: func(clix *cli.Context) error {
		id := clix.Args().First()
		container, err := cfg.Client().LoadContainer(cfg.Context(), id)
		if err != nil {
			return err
		}
		ctx := cfg.Context()
		if err := systemd.Stop(ctx, id); err != nil {
			return err
		}
		if err := systemd.Disable(ctx, id); err != nil {
			return err
		}
		return container.Delete(ctx, containerd.WithSnapshotCleanup)
	},
}
