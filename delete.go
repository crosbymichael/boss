package main

import (
	"github.com/containerd/containerd"
	"github.com/crosbymichael/boss/system"
	"github.com/crosbymichael/boss/systemd"
	"github.com/urfave/cli"
)

var deleteCommand = cli.Command{
	Name:  "delete",
	Usage: "delete a service",
	Action: func(clix *cli.Context) error {
		id := clix.Args().First()
		ctx := system.Context()
		client, err := system.NewClient()
		if err != nil {
			return err
		}
		defer client.Close()
		container, err := client.LoadContainer(ctx, id)
		if err != nil {
			return err
		}
		if err := systemd.Stop(ctx, id); err != nil {
			return err
		}
		if err := systemd.Disable(ctx, id); err != nil {
			return err
		}
		c, err := system.Load()
		if err != nil {
			return err
		}
		register, err := system.GetRegister(c)
		if err != nil {
			return err
		}
		config, err := getConfig(ctx, container)
		if err != nil {
			return err
		}
		network, err := system.GetNetwork(c, config.Network)
		if err != nil {
			return err
		}
		if err := network.Remove(ctx, container); err != nil {
			return err
		}
		for name := range config.Services {
			if err := register.Deregister(id, name); err != nil {
				return err
			}
		}
		return container.Delete(ctx, containerd.WithSnapshotCleanup)
	},
}
