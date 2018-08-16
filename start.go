package main

import (
	"github.com/crosbymichael/boss/api/v1"
	"github.com/urfave/cli"
)

var startCommand = cli.Command{
	Name:  "start",
	Usage: "start an existing service",
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
		_, err = agent.Start(ctx, &v1.StartRequest{
			ID: id,
		})
		return err
	},
}
