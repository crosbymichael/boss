package main

import (
	"github.com/crosbymichael/boss/agent"
	"github.com/crosbymichael/boss/config"
	"github.com/crosbymichael/boss/system"
	"github.com/urfave/cli"
)

var agentCommand = cli.Command{
	Name:  "agent",
	Usage: "run the boss agent",
	Action: func(clix *cli.Context) error {
		c, err := config.Load()
		if err != nil {
			return err
		}
		client, err := system.NewClient()
		if err != nil {
			return err
		}
		defer client.Close()
		store, err := c.Store()
		if err != nil {
			return err
		}
		_, err = agent.New(c, client, store)
		if err != nil {
			return err
		}
		return nil
	},
}
