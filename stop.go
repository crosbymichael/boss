package main

import (
	"github.com/crosbymichael/boss/system"
	"github.com/crosbymichael/boss/systemd"
	"github.com/urfave/cli"
)

var stopCommand = cli.Command{
	Name:  "stop",
	Usage: "stop a running service",
	Action: func(clix *cli.Context) error {
		var (
			id  = clix.Args().First()
			ctx = system.Context()
		)
		return systemd.Stop(ctx, id)
	},
}
