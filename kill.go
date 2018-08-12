package main

import (
	"github.com/crosbymichael/boss/config"
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
		defer client.Close()
		container, err := client.LoadContainer(ctx, id)
		if err != nil {
			return err
		}
		config, err := config.GetConfig(ctx, container)
		if err != nil {
			return err
		}
		register, err := system.GetRegister(c)
		if err != nil {
			return err
		}
		for name := range config.Services {
			if err := register.EnableMaintainance(id, name, "manual kill"); err != nil {
				return err
			}
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
