package main

import (
	"os"

	"github.com/BurntSushi/toml"
	"github.com/containerd/containerd/platforms"
	"github.com/crosbymichael/boss/config"
	"github.com/crosbymichael/boss/flux"
	"github.com/crosbymichael/boss/system"
	"github.com/crosbymichael/boss/systemd"
	"github.com/urfave/cli"
)

var createCommand = cli.Command{
	Name:  "create",
	Usage: "create a container",
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
		ctx := system.Context()
		client, err := system.NewClient()
		if err != nil {
			return err
		}
		defer client.Close()
		image, err := getImage(ctx, client, container.Image, clix, os.Stdout, true)
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
