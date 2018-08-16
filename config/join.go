package config

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/hashicorp/consul/api"
	"github.com/urfave/cli"
)

type Join struct {
	IPs []string
}

func (s *Join) Name() string {
	return "join"
}

func (s *Join) Run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	consul, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return err
	}
	for _, ip := range s.IPs {
		if err := consul.Agent().Join(ip, false); err != nil {
			return err
		}
	}
	return nil
}

func (s *Join) Remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	consul, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return err
	}
	return consul.Agent().Leave()
}
