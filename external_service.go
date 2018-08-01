package main

import (
	"github.com/BurntSushi/toml"
	"github.com/urfave/cli"
)

type ExternalService struct {
	ID       string             `toml:"id"`
	IP       string             `toml:"ip"`
	Services map[string]Service `toml:"services"`
}

var servicesCommand = cli.Command{
	Name:  "services",
	Usage: "manage services",
	Subcommands: []cli.Command{
		addServiceCommand,
	},
}

var addServiceCommand = cli.Command{
	Name:  "add",
	Usage: "add external services",
	Action: func(clix *cli.Context) error {
		var es ExternalService
		if _, err := toml.DecodeFile(clix.Args().First(), &es); err != nil {
			return err
		}
		for name, s := range es.Services {
			if err := register.Register(es.ID, name, es.IP, s); err != nil {
				return err
			}
		}
		return nil
	},
}
