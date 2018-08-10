package main

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/urfave/cli"
	"golang.org/x/sys/unix"
)

var networkCommand = cli.Command{
	Name:   "network",
	Usage:  "internal network commands",
	Hidden: true,
	Subcommands: []cli.Command{
		networkCreateCommand,
	},
}

var networkCreateCommand = cli.Command{
	Name:  "create",
	Usage: "create a new network namespace",
	Action: func(clix *cli.Context) error {
		path := clix.Args().First()
		if path == "" {
			return errors.New("netns path required")
		}
		if err := os.MkdirAll(filepath.Dir(path), 0711); err != nil {
			return err
		}
		return unix.Mount("/proc/self/ns/net", path, "none", unix.MS_BIND, "")
	},
}
