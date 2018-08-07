package main

import (
	"github.com/containerd/containerd"
	"github.com/urfave/cli"
)

var startCommand = cli.Command{
	Name:   "start",
	Usage:  "start an existing service",
	Before: ReadyBefore,
	Action: func(clix *cli.Context) error {
		var (
			id     = clix.Args().First()
			ctx    = cfg.Context()
			client = cfg.Client()
		)
		container, err := client.LoadContainer(ctx, id)
		if err != nil {
			return err
		}
		return container.Update(ctx, withStatus(containerd.Running))
	},
}
