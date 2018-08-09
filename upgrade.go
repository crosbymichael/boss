package main

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/platforms"
	"github.com/crosbymichael/boss/flux"
	"github.com/crosbymichael/boss/system"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/sys/unix"
)

var upgradeCommand = cli.Command{
	Name:  "upgrade",
	Usage: "upgrade a container's image but keep its data, like it should be",
	Flags: []cli.Flag{
		cli.StringSliceFlag{
			Name:  "platform",
			Usage: "pull content from a specific platform",
			Value: &cli.StringSlice{platforms.Default()},
		},
		cli.BoolFlag{
			Name:  "all-platforms",
			Usage: "pull content from all platforms",
		},
	},
	Action: func(clix *cli.Context) error {
		var (
			id  = clix.Args().First()
			ref = clix.Args().Get(1)
			ctx = system.Context()
		)
		client, err := system.NewClient()
		if err != nil {
			return err
		}
		defer client.Close()
		ctx, done, err := client.WithLease(ctx)
		if err != nil {
			return err
		}
		defer done(ctx)
		image, err := getImage(ctx, client, ref, clix, true)
		if err != nil {
			return err
		}
		return pauseAndRun(ctx, id, client, func() error {
			container, err := client.LoadContainer(ctx, id)
			if err != nil {
				return err
			}
			if err := container.Update(ctx, flux.WithUpgrade(image)); err != nil {
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

func pauseAndRun(ctx context.Context, id string, client *containerd.Client, fn func() error) error {
	c, err := system.Load()
	if err != nil {
		return err
	}
	register, err := system.GetRegister(c)
	if err != nil {
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
	if err := register.EnableMaintainance(id, "upgrade image"); err != nil {
		return err
	}
	defer func() {
		if err := register.DisableMaintainance(id); err != nil {
			logrus.WithError(err).Error("disable maintaince")
		}
	}()
	if err := task.Pause(ctx); err != nil {
		return err
	}
	defer task.Resume(ctx)
	return fn()
}

func withImage(i containerd.Image) containerd.UpdateContainerOpts {
	return func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
		c.Image = i.Name()
		return nil
	}
}
