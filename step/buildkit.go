package step

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/crosbymichael/boss/config"
	"github.com/urfave/cli"
)

const buildkitUnit = `[Unit]
Description=buildkit
Documentation=moby/buildkit
After=containerd.service network.target

[Service]
ExecStart=/opt/containerd/bin/buildkitd --containerd-worker=true --oci-worker=false --addr tcp://0.0.0.0:9500
Restart=always

[Install]
WantedBy=multi-user.target`

type Buildkit struct {
	Config *config.Config
}

func (s *Buildkit) Name() string {
	return "buildkit"
}

func (s *Buildkit) Run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	const name = "buildkit.service"
	if err := install(ctx, client, s.Config.Buildkit.Image, clix); err != nil {
		return err
	}
	if err := writeUnit(name, buildkitUnit); err != nil {
		return err
	}
	return startNewService(ctx, name)
}

func (s *Buildkit) Remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	if err := client.ImageService().Delete(ctx, s.Config.Buildkit.Image); err != nil {
		return err
	}
	const name = "buildkit.service"
	return disableService(ctx, name)
}
