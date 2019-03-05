package main

import (
	"os"

	"github.com/urfave/cli"
)

var dhcpCommand = cli.Command{
	Name:  "dhcp",
	Usage: "dhcp daemon",
	Action: func(clix *cli.Context) error {
		if err := os.Remove(defaultSocketPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return runDaemon(defaultSocketPath)
	},
}
