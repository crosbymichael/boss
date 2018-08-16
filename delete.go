package main

import (
	"github.com/crosbymichael/boss/api/v1"
	"github.com/urfave/cli"
)

var deleteCommand = cli.Command{
	Name:  "delete",
	Usage: "delete a service",
	Action: func(clix *cli.Context) error {
		id := clix.Args().First()
		ctx := Context()
		agent, err := Agent(clix)
		if err != nil {
			return err
		}
		defer agent.Close()
		_, err = agent.Delete(ctx, &v1.DeleteRequest{
			ID: id,
		})
		return err
	},
}
