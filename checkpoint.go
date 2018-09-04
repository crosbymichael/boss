package main

import (
	"github.com/crosbymichael/boss/api/v1"
	"github.com/urfave/cli"
)

var checkpointCommand = cli.Command{
	Name:  "checkpoint",
	Usage: "checkpoint a container",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "live",
			Usage: "enable live checkpoint(criu must be installed)",
		},
		cli.BoolFlag{
			Name:  "exit",
			Usage: "exit the container after a successful checkpoint",
		},
		cli.StringFlag{
			Name:  "ref",
			Usage: "ref name of the created checkpoint",
		},
	},
	Action: func(clix *cli.Context) error {
		ctx := Context()
		agent, err := Agent(clix)
		if err != nil {
			return err
		}
		defer agent.Close()
		_, err = agent.Checkpoint(ctx, &v1.CheckpointRequest{
			ID:   clix.Args().First(),
			Ref:  clix.String("ref"),
			Live: clix.Bool("live"),
			Exit: clix.Bool("exit"),
		})
		return err
	},
}
