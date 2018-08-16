package main

import (
	"encoding/json"
	"os"

	"github.com/crosbymichael/boss/api/v1"
	"github.com/urfave/cli"
)

var getCommand = cli.Command{
	Name:  "get",
	Usage: "get the config of a container",
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
		r, err := agent.Get(ctx, &v1.GetRequest{
			ID: id,
		})
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(r.Container)
	},
}
