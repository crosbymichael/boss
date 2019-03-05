package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/crosbymichael/boss/cmd"
	bv "github.com/crosbymichael/boss/version"
	raven "github.com/getsentry/raven-go"
	"github.com/urfave/cli"
)

func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

func main() {
	switch os.Args[0] {
	case "macvlan":
		skel.PluginMain(macvlanAdd, macvlanDelete, version.All)
	case "dhcp":
		skel.PluginMain(dhcpAdd, dhcpDelete, version.All)
	default:
		app := cli.NewApp()
		app.Name = "boss-network"
		app.Version = bv.Version
		app.Usage = "run containers like a ross"
		app.Description = cmd.Banner
		app.Flags = []cli.Flag{
			cli.BoolFlag{
				Name:  "debug",
				Usage: "enable debug output in the logs",
			},
			cli.StringFlag{
				Name:   "sentry-dsn",
				Usage:  "sentry DSN",
				EnvVar: "SENTRY_DSN",
			},
		}
		app.Before = func(clix *cli.Context) error {
			if dsn := clix.GlobalString("sentry-dsn"); dsn != "" {
				raven.SetDSN(dsn)
				raven.DefaultClient.SetRelease(bv.Version)
			}
			return nil
		}
		app.Commands = []cli.Command{
			networkCreateCommand,
			dhcpCommand,
		}
		if err := app.Run(os.Args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			raven.CaptureErrorAndWait(err, nil)
			os.Exit(1)
		}
	}
}
