package main

import (
	"github.com/crosbymichael/boss/api/v1"
	"github.com/urfave/cli"
)

var rollbackCommand = cli.Command{
	Name:  "rollback",
	Usage: "rollback a container to a previous revision",
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
		_, err = agent.Rollback(ctx, &v1.RollbackRequest{
			ID: id,
		})
		return err
	},
}
