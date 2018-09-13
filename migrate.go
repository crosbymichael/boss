package main

import (
	"github.com/crosbymichael/boss/api/v1"
	"github.com/urfave/cli"
)

var migrateCommand = cli.Command{
	Name:  "migrate",
	Usage: "migrate a container from one agent to another",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "live",
			Usage: "enable live checkpoint(criu must be installed)",
		},
		cli.BoolFlag{
			Name:  "stop",
			Usage: "stop the container after a successful checkpoint",
		},
		cli.BoolFlag{
			Name:  "delete",
			Usage: "delete the container on the local agent after a successful checkpoint",
		},

		cli.StringFlag{
			Name:  "ref",
			Usage: "ref name of the created checkpoint",
		},
		cli.StringFlag{
			Name:  "to",
			Usage: "destination agent",
		},
	},

	Action: func(clix *cli.Context) error {
		ctx := Context()
		agent, err := Agent(clix)
		if err != nil {
			return err
		}
		defer agent.Close()
		_, err = agent.Migrate(ctx, &v1.MigrateRequest{
			ID:     clix.Args().First(),
			Ref:    clix.String("ref"),
			Stop:   clix.Bool("stop"),
			Delete: clix.Bool("delete"),
			To:     clix.String("to"),
			Live:   clix.Bool("live"),
		})
		return err
	},
}
