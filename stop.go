package main

import (
	"github.com/containerd/containerd"
	"github.com/urfave/cli"
)

var stopCommand = cli.Command{
	Name:   "stop",
	Usage:  "stop a running service",
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
		task, err := container.Task(ctx, nil)
		if err != nil {
			return err
		}
		wait, err := task.Wait(ctx)
		if err != nil {
			return err
		}
		if err := container.Update(ctx, withStatus(containerd.Stopped)); err != nil {
			return err
		}
		<-wait
		return nil
	},
}
