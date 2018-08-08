package main

import (
	"github.com/crosbymichael/boss/monitor"
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
		return container.Update(cfg.Context(), withStatus(monitor.DeleteStatus))
	},
}
