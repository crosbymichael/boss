package main

import (
	"github.com/crosbymichael/boss/system"
	"github.com/urfave/cli"
	"golang.org/x/sys/unix"
)

var killCommand = cli.Command{
	Name:  "kill",
	Usage: "kill a running service",
	Action: func(clix *cli.Context) error {
		var (
			id  = clix.Args().First()
			ctx = system.Context()
		)
		c, err := system.Load()
		if err != nil {
			return err
		}
		client, err := system.NewClient()
		if err != nil {
			return err
		}
		client.Close()
		register, err := system.GetRegister(c)
		if err != nil {
			return err
		}
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
