package main

import (
	"github.com/crosbymichael/boss/systemd"
	"github.com/urfave/cli"
)

var stopCommand = cli.Command{
	Name:   "stop",
	Usage:  "stop a running service",
	Before: ReadyBefore,
	Action: func(clix *cli.Context) error {
		var (
			id  = clix.Args().First()
			ctx = cfg.Context()
		)
		return systemd.Stop(ctx, id)
	},
}
