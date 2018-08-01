package main

import (
	"fmt"
	"os"

	"github.com/hashicorp/consul/api"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var register Register

func main() {
	app := cli.NewApp()
	app.Name = "boss"
	app.Version = "3"
	app.Usage = "simple container services for me"
	app.Description = "run containers like a boss or rick ross"
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enable debug output",
		},
		cli.StringFlag{
			Name:  "namespace,n",
			Usage: "containerd namespace",
			Value: "default",
		},
		cli.StringFlag{
			Name:  "register",
			Usage: "register for services(consul,none)",
		},
	}
	app.Commands = []cli.Command{
		agentCommand,
		buildCommand,
		createCommand,
		deleteCommand,
		killCommand,
		rollbackCommand,
		servicesCommand,
		startCommand,
		stopCommand,
		upgradeCommand,
	}
	app.Before = func(clix *cli.Context) error {
		if clix.GlobalBool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		switch clix.GlobalString("register") {
		case "consul":
			consul, err := api.NewClient(api.DefaultConfig())
			if err != nil {
				return err
			}
			register = &Consul{
				client: consul,
			}
		default:
			register = &nullRegister{}
		}
		return nil
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
}
