package main

import (
	"github.com/BurntSushi/toml"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/platforms"
	"github.com/crosbymichael/boss/config"
	"github.com/crosbymichael/boss/system"
	"github.com/urfave/cli"
	"golang.org/x/sys/unix"
)

var updateCommand = cli.Command{
	Name:  "update",
	Usage: "update an existing container's configuration",
	Flags: []cli.Flag{
		cli.StringSliceFlag{
			Name:  "platform",
			Usage: "pull content from a specific platform",
			Value: &cli.StringSlice{platforms.Default()},
		},
		cli.BoolFlag{
			Name:  "all-platforms",
			Usage: "pull content from all platforms",
		},
		cli.BoolFlag{
			Name:  "plain-http",
			Usage: "don't fetch with https",
		},
	},
	Action: func(clix *cli.Context) error {
		var (
			path = clix.Args().First()
			ctx  = system.Context()
		)
		var newConfig Container
		if _, err := toml.DecodeFile(path, &newConfig); err != nil {
			return err
		}
		client, err := system.NewClient()
		if err != nil {
			return err
		}
		defer client.Close()

		ctx, done, err := client.WithLease(ctx)
		if err != nil {
			return err
		}
		defer done(ctx)
		container, err := client.LoadContainer(ctx, newConfig.ID)
		if err != nil {
			return err
		}

		c, err := config.Load()
		if err != nil {
			return err
		}
		register, err := c.GetRegister()
		if err != nil {
			return err
		}
		store, err := c.Store()
		if err != nil {
			return err
		}
		current, err := config.GetConfig(ctx, container)
		if err != nil {
			return err
		}
		// set all current services into maintaince mode
		for name := range current.Services {
			if err := register.EnableMaintainance(container.ID(), name, "update container configuration"); err != nil {
				return err
			}
		}
		var changes []change
		for name := range current.Services {
			if _, ok := newConfig.Services[name]; !ok {
				// if the new config does not have a service, deregister the old one
				changes = append(changes, &deregisterChange{
					register: register,
					name:     name,
				})
			}
		}
		changes = append(changes, &imageUpdateChange{
			ref:    newConfig.Image,
			clix:   clix,
			client: client,
		})
		changes = append(changes, &configChange{
			client: client,
			c:      newConfig.Proto(),
		})
		changes = append(changes, &filesChange{
			c:     newConfig.Proto(),
			store: store,
		})
		return pauseAndRun(ctx, container, func() error {
			for _, ch := range changes {
				if err := ch.update(ctx, container); err != nil {
					return err
				}
			}
			// bump the task to pickup the changes
			task, err := container.Task(ctx, nil)
			if err != nil {
				if errdefs.IsNotFound(err) {
					return nil
				}
				return err
			}
			return task.Kill(ctx, unix.SIGTERM)
		})
	},
}
