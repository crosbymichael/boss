package main

import (
	"fmt"
	"os"

	"github.com/crosbymichael/boss/cmd"
	"github.com/crosbymichael/boss/version"
	raven "github.com/getsentry/raven-go"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "boss-systemd"
	app.Version = version.Version
	app.Usage = "run containers like a ross"
	app.Description = cmd.Banner
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
		systemdExecStartPreCommand,
		systemdExecStartCommand,
		systemdExecStopPostCommand,
		networkCommand,
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		raven.CaptureErrorAndWait(err, nil)
		os.Exit(1)
	}
}
