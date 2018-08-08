package main

import (
	"github.com/crosbymichael/boss/flux"
	"github.com/urfave/cli"
	"golang.org/x/sys/unix"
)

var rollbackCommand = cli.Command{
	Name:   "rollback",
	Usage:  "rollback a container to a previous revision",
	Before: ReadyBefore,
	Action: func(clix *cli.Context) error {
		var (
			id     = clix.Args().First()
			ctx    = cfg.Context()
			client = cfg.Client()
		)
		ctx, done, err := client.WithLease(ctx)
		if err != nil {
			return err
		}
		defer done(ctx)
		container, err := client.LoadContainer(ctx, id)
		if err != nil {
			return err
		}
		return pauseAndRun(ctx, id, client, func() error {
			if err := container.Update(ctx, flux.WithRollback); err != nil {
				return err
			}
			task, err := container.Task(ctx, nil)
			if err != nil {
				return err
			}
			return task.Kill(ctx, unix.SIGTERM)
		})
	},
}
