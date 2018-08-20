package main

import (
	"context"
	"fmt"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/typeurl"
	"github.com/crosbymichael/boss/opts"
	"github.com/crosbymichael/boss/system"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var migrateCommand = cli.Command{
	Name:   "migrate",
	Usage:  "migrate boss < 9 to 12",
	Hidden: true,
	Action: func(clix *cli.Context) error {
		typeurl.Register(&Container{}, "io.boss.v1.Container")
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
			if _, err := opts.GetConfig(ctx, c); err == nil {
				continue
			}
			fmt.Println("migrating", c.ID())
			current, err := loadOldConfig(ctx, c)
			if err != nil {
				return errors.Wrap(err, "load old config")
			}
			if err := c.Update(ctx, withMigrate(current)); err != nil {
				return errors.Wrap(err, "migrate")
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
		c.Extensions[opts.LastConfig] = *any
		c.Extensions[opts.CurrentConfig] = *any
		return nil
	}
}

func loadOldConfig(ctx context.Context, container containerd.Container) (*Container, error) {
	info, err := container.Info(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "load info")
	}
	d := info.Extensions[opts.CurrentConfig]
	v, err := typeurl.UnmarshalAny(&d)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshal any")
	}
	c, ok := v.(*Container)
	if !ok {
		return nil, errors.New("not old format")
	}
	return c, nil
}
