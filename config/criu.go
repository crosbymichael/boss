package config

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/urfave/cli"
)

type Criu struct {
	Image    string `toml:"image"`
	Iptables string `toml:"iptables"`
}

func (s *Criu) Name() string {
	return "criu"
}

func (s *Criu) Run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	if err := install(ctx, client, s.Image, clix); err != nil {
		return err
	}
	if s.Iptables != "" {
		if err := install(ctx, client, s.Iptables, clix); err != nil {
			return err
		}
	}
	return nil
}

func (s *Criu) Remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	return nil
}
