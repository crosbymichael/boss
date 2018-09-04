package main

import (
	"context"
	"fmt"
	"os"

	"github.com/containerd/containerd/namespaces"
	"github.com/crosbymichael/boss/api"
	"github.com/crosbymichael/boss/api/v1"
	"github.com/crosbymichael/boss/version"
	raven "github.com/getsentry/raven-go"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var Version string

func main() {
	app := cli.NewApp()
	app.Name = "boss"
	// haha semver
	app.Version = version.Version
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
		cli.StringFlag{
			Name:   "sentry-dsn",
			Usage:  "sentry DSN",
			EnvVar: "SENTRY_DSN",
		},
	}
	app.Before = func(clix *cli.Context) error {
		if clix.GlobalBool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		if dsn := clix.GlobalString("sentry-dsn"); dsn != "" {
			raven.SetDSN(dsn)
			raven.DefaultClient.SetRelease(version.Version)
		}
		return nil
	}
	app.Commands = []cli.Command{
		agentCommand,
		buildCommand,
		checkpointCommand,
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
		raven.CaptureErrorAndWait(err, nil)
		os.Exit(1)
	}
}

func Context() context.Context {
	return namespaces.WithNamespace(context.Background(), v1.DefaultNamespace)
}

func Agent(clix *cli.Context) (*api.LocalAgent, error) {
	return api.Agent(clix.GlobalString("agent"))
}
