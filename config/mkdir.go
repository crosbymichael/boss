package config

import (
	"context"
	"os"

	"github.com/containerd/containerd"
	"github.com/urfave/cli"
)

type Mkdir struct {
}

func (s *Mkdir) Name() string {
	return "mkdir /var/lib/boss"
}

func (s *Mkdir) Run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	return os.MkdirAll(Root, 0711)
}

func (s *Mkdir) Remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	return os.RemoveAll(Root)
}
