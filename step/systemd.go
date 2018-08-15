package step

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/crosbymichael/boss/systemd"
	"github.com/urfave/cli"
)

type Systemd struct {
}

func (s *Systemd) Name() string {
	return "systemd"
}

func (s *Systemd) Run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	return systemd.Install()
}

func (s *Systemd) Remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	return systemd.Remove()
}
