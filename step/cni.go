package step

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/crosbymichael/boss/config"
	"github.com/urfave/cli"
)

type CNI struct {
	Config *config.Config
}

func (s *CNI) Name() string {
	return "cni"
}

func (s *CNI) Run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	return install(ctx, client, s.Config.CNI.Image, clix)
}

func (s *CNI) Remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	if err := client.ImageService().Delete(ctx, s.Config.CNI.Image); err != nil {
		return err
	}
	return nil
}
