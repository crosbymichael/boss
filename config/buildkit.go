package config

import (
	"context"

	"github.com/containerd/containerd"
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
	Image string `toml:"image"`
}

func (s *Buildkit) Name() string {
	return "buildkit"
}

func (s *Buildkit) Run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	const name = "buildkit.service"
	if err := install(ctx, client, s.Image, clix); err != nil {
		return err
	}
	if err := writeUnit(name, buildkitUnit); err != nil {
		return err
	}
	return startNewService(ctx, name)
}

func (s *Buildkit) Remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	if err := client.ImageService().Delete(ctx, s.Image); err != nil {
		return err
	}
	const name = "buildkit.service"
	return disableService(ctx, name)
}
