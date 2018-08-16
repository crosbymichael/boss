package main

import (
	"github.com/BurntSushi/toml"
	"github.com/containerd/containerd/platforms"
	"github.com/crosbymichael/boss/api/v1"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
)

var createCommand = cli.Command{
	Name:  "create",
	Usage: "create a container",
	Flags: []cli.Flag{
		cli.StringSliceFlag{
			Name:  "platform",
			Usage: "pull content from a specific platform",
			Value: &cli.StringSlice{platforms.Default()},
		},
		cli.BoolFlag{
			Name:  "all-platforms",
			Usage: "pull content from all platforms",
		},
		cli.BoolFlag{
			Name:  "plain-http",
			Usage: "don't fetch with https",
		},
	},
	Action: func(clix *cli.Context) error {
		var container Container
		if _, err := toml.DecodeFile(clix.Args().First(), &container); err != nil {
			return err
		}
		agent, err := Agent(clix)
		if err != nil {
			return err
		}
		_, err = agent.Create(Context(), &v1.CreateRequest{
			Container: container.Proto(),
		})
		return err
	},
}

func Agent(clix *cli.Context) (v1.AgentClient, error) {
	conn, err := grpc.Dial(clix.GlobalString("agent"), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	// TODO close conn
	return v1.NewAgentClient(conn), nil
}
