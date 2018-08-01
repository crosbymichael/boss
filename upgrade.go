package main

import "github.com/urfave/cli"

var upgradeCommand = cli.Command{
	Name:  "upgrade",
	Usage: "upgrade a container's image but keep its data, like it should be",
	Action: func(clix *cli.Context) error {
		return nil
	},
}
