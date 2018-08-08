package main

import (
	"github.com/crosbymichael/boss/systemd"
	"github.com/urfave/cli"
)

var startCommand = cli.Command{
	Name:   "start",
	Usage:  "start an existing service",
	Before: ReadyBefore,
	Action: func(clix *cli.Context) error {
		var (
			id  = clix.Args().First()
			ctx = cfg.Context()
		)
		return systemd.Start(ctx, id)
	},
}
