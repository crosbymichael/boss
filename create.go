package main

import (
	"github.com/BurntSushi/toml"
	"github.com/crosbymichael/boss/api/v1"
	"github.com/crosbymichael/boss/cmd"
	"github.com/urfave/cli"
)

var createCommand = cli.Command{
	Name:  "create",
	Usage: "create a container",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "update",
			Usage: "create or update",
		},
	},
	Action: func(clix *cli.Context) error {
		var container cmd.Container
		if _, err := toml.DecodeFile(clix.Args().First(), &container); err != nil {
			return err
		}
		agent, err := Agent(clix)
		if err != nil {
			return err
		}
		defer agent.Close()
		_, err = agent.Create(Context(), &v1.CreateRequest{
			Container: container.Proto(),
			Update:    clix.Bool("update"),
		})
		return err
	},
}
