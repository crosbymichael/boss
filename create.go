package main

import (
	"github.com/BurntSushi/toml"
	"github.com/containerd/containerd/platforms"
	"github.com/crosbymichael/boss/config"
	"github.com/crosbymichael/boss/flux"
	"github.com/crosbymichael/boss/systemd"
	"github.com/urfave/cli"
)

var createCommand = cli.Command{
	Name:   "create",
	Usage:  "create a container",
	Before: ReadyBefore,
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
		var container config.Container
		if _, err := toml.DecodeFile(clix.Args().First(), &container); err != nil {
			return err
		}
		var (
			ctx    = cfg.Context()
			client = cfg.Client()
		)
		image, err := getImage(ctx, client, container.Image, clix)
		if err != nil {
			return err
		}
		if _, err := client.NewContainer(
			ctx,
			container.ID,
			config.WithBossConfig(&container, image),
			flux.WithNewSnapshot(image),
		); err != nil {
			return err
		}
		if err := systemd.Enable(ctx, container.ID); err != nil {
			return err
		}
		return systemd.Start(ctx, container.ID)
	},
}
