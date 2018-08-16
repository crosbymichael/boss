package main

import (
	"github.com/crosbymichael/boss/api/v1"
	"github.com/urfave/cli"
)

var stopCommand = cli.Command{
	Name:  "stop",
	Usage: "stop a running service",
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
		_, err = agent.Stop(ctx, &v1.StopRequest{
			ID: id,
		})
		return err
	},
}
