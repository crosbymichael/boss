package main

import (
	"errors"
	"os"

	"github.com/urfave/cli"
	"golang.org/x/sys/unix"
)

var networkCreateCommand = cli.Command{
	Name:  "create",
	Usage: "create a new network namespace",
	Action: func(clix *cli.Context) error {
		path := clix.Args().First()
		if path == "" {
			return errors.New("netns path required")
		}
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		return unix.Mount("/proc/self/ns/net", path, "none", unix.MS_BIND, "")
	},
}
