package main

import (
	"github.com/urfave/cli"
	"golang.org/x/sys/unix"
)

var killCommand = cli.Command{
	Name:   "kill",
	Usage:  "kill a running service",
	Before: ReadyBefore,
	Action: func(clix *cli.Context) error {
		var (
			id  = clix.Args().First()
			ctx = cfg.Context()
		)
		if err := cfg.GetRegister().EnableMaintainance(id, "manual kill"); err != nil {
			return err
		}
		container, err := cfg.Client().LoadContainer(ctx, id)
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
