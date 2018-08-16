package main

import (
	"github.com/crosbymichael/boss/api/v1"
	"github.com/urfave/cli"
)

var killCommand = cli.Command{
	Name:  "kill",
	Usage: "kill a running service",
	Action: func(clix *cli.Context) error {
		var (
			id  = clix.Args().First()
			ctx = Context()
		)

		agent, err := Agent(clix)
		if err != nil {
			return err
		}
		defer agent.Close()
		_, err = agent.Kill(ctx, &v1.KillRequest{
			ID: id,
		})
		return err
	},
}
