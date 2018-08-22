package main

import (
	"github.com/BurntSushi/toml"
	"github.com/crosbymichael/boss/api/v1"
	"github.com/crosbymichael/boss/cmd"
	"github.com/urfave/cli"
)

var updateCommand = cli.Command{
	Name:  "update",
	Usage: "update an existing container's configuration",
	Action: func(clix *cli.Context) error {
		var (
			path = clix.Args().First()
			ctx  = Context()
		)
		var newConfig cmd.Container
		if _, err := toml.DecodeFile(path, &newConfig); err != nil {
			return err
		}
		agent, err := Agent(clix)
		if err != nil {
			return err
		}
		defer agent.Close()
		_, err = agent.Update(ctx, &v1.UpdateRequest{
			Container: newConfig.Proto(),
		})
		return err
	},
}
