package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/typeurl"
	"github.com/crosbymichael/boss/config"
	"github.com/crosbymichael/boss/system"
	"github.com/urfave/cli"
)

func init() {
	typeurl.Register(&Container{}, "io.boss.v1.Container")
}

var migrateCommand = cli.Command{
	Name:   "migrate",
	Usage:  "migrate boss < 9 to 12",
	Hidden: true,
	Action: func(clix *cli.Context) error {
		ctx := Context()
		client, err := system.NewClient()
		if err != nil {
			return err
		}
		defer client.Close()
		containers, err := client.Containers(ctx)
		if err != nil {
			return err
		}
		for _, c := range containers {
			if _, err := config.GetConfig(ctx, c); err == nil {
				continue
			}
			fmt.Println("migrating", c.ID())
			current, err := loadOldConfig(ctx, c)
			if err != nil {
				return err
			}
			if err := c.Update(ctx, withMigrate(current)); err != nil {
				return err
			}
		}
		return nil
	},
}

func withMigrate(current *Container) func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
	return func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
		data := current.Proto()
		any, err := typeurl.MarshalAny(data)
		if err != nil {
			return err
		}
		c.Extensions[config.LastConfig] = *any
		c.Extensions[config.CurrentConfig] = *any
		return nil
	}
}

func loadOldConfig(ctx context.Context, container containerd.Container) (*Container, error) {
	info, err := container.Info(ctx)
	if err != nil {
		return nil, err
	}
	d := info.Extensions[config.CurrentConfig]
	v, err := typeurl.UnmarshalAny(&d)
	if err != nil {
		return nil, err
	}
	c, ok := v.(*Container)
	if !ok {
		return nil, errors.New("not old format")
	}
	return c, nil
}
