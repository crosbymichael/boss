package main

import (
	"os"

	"github.com/BurntSushi/toml"
	"github.com/crosbymichael/boss/config"
	"github.com/crosbymichael/boss/system"
	"github.com/urfave/cli"
)

var getCommand = cli.Command{
	Name:  "get",
	Usage: "get the config of a container",
	Action: func(clix *cli.Context) error {
		var (
			id  = clix.Args().First()
			ctx = system.Context()
		)
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

		return toml.NewEncoder(os.Stdout).Encode(config)
	},
}
