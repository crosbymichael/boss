package main

import (
	"context"
	"fmt"
	"os"

	"github.com/containerd/containerd/namespaces"
	"github.com/crosbymichael/boss/api/v1"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "boss"
	app.Version = "13-dev"
	app.Usage = "run containers like a ross"
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
			Usage: "enable debug output in the logs",
		},
		cli.StringFlag{
			Name:   "agent",
			Usage:  "agent address",
			Value:  "0.0.0.0:1337",
			EnvVar: "BOSS_AGENT",
		},
	}
	app.Before = func(clix *cli.Context) error {
		if clix.GlobalBool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}
	app.Commands = []cli.Command{
		agentCommand,
		buildCommand,
		createCommand,
		deleteCommand,
		getCommand,
		initCommand,
		killCommand,
		listCommand,
		migrateCommand,
		networkCommand,
		rollbackCommand,
		startCommand,
		stopCommand,
		systemdCommand,
		updateCommand,
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func Context() context.Context {
	return namespaces.WithNamespace(context.Background(), v1.DefaultNamespace)
}
