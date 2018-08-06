package main

import (
	"fmt"
	"os"

	"github.com/crosbymichael/boss/config"
	"github.com/crosbymichael/boss/monitor"
	"github.com/hashicorp/consul/api"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var register monitor.Register

func main() {
	app := cli.NewApp()
	app.Name = "boss"
	app.Version = "5"
	app.Usage = "container tools for my setup"
	app.Description = `

                    ___           ___           ___     
     _____         /\  \         /\__\         /\__\    
    /::\  \       /::\  \       /:/ _/_       /:/ _/_   
   /:/\:\  \     /:/\:\  \     /:/ /\  \     /:/ /\  \  
  /:/ /::\__\   /:/  \:\  \   /:/ /::\  \   /:/ /::\  \ 
 /:/_/:/\:|__| /:/__/ \:\__\ /:/_/:/\:\__\ /:/_/:/\:\__\
 \:\/:/ /:/  / \:\  \ /:/  / \:\/:/ /:/  / \:\/:/ /:/  /
  \::/_/:/  /   \:\  /:/  /   \::/ /:/  /   \::/ /:/  / 
   \:\/:/  /     \:\/:/  /     \/_/:/  /     \/_/:/  /  
    \::/  /       \::/  /        /:/  /        /:/  /   
     \/__/         \/__/         \/__/         \/__/    

run containers like a boss`
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
			Name:   "register",
			Usage:  "register for services(consul,none)",
			Value:  "none",
			EnvVar: "BOSS_REGISTER",
		},
	}
	app.Commands = []cli.Command{
		agentCommand,
		buildCommand,
		createCommand,
		deleteCommand,
		initCommand,
		killCommand,
		listCommand,
		rollbackCommand,
		startCommand,
		stopCommand,
		upgradeCommand,
	}
	app.Before = func(clix *cli.Context) error {
		if clix.GlobalBool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		if err := os.MkdirAll(config.Root, 0711); err != nil {
			return err
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
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
