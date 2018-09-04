package main

import (
	"github.com/crosbymichael/boss/api/v1"
	"github.com/urfave/cli"
)

var restoreCommand = cli.Command{
	Name:  "restore",
	Usage: "restore a container",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "live",
			Usage: "enable live restore(criu must be installed)",
		},
	},
	Action: func(clix *cli.Context) error {
		ctx := Context()
		agent, err := Agent(clix)
		if err != nil {
			return err
		}
		defer agent.Close()
		_, err = agent.Restore(ctx, &v1.RestoreRequest{
			Ref:  clix.Args().First(),
			Live: clix.Bool("live"),
		})
		return err
	},
}
