package main

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
ExecStart=/opt/containerd/bin/buildkitd --containerd-worker=true --oci-worker=false
Restart=always

[Install]
WantedBy=multi-user.target`

type buildkitStep struct {
	config *config.Config
}

func (s *buildkitStep) name() string {
	return "buildkit"
}

func (s *buildkitStep) run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	const name = "buildkit.service"
	if err := install(ctx, client, s.config.Buildkit.Image, clix); err != nil {
		return err
	}
	if err := writeUnit(name, buildkitUnit); err != nil {
		return err
	}
	return startNewService(ctx, name)
}

func (s *buildkitStep) remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	if err := client.ImageService().Delete(ctx, s.config.Buildkit.Image); err != nil {
		return err
	}
	const name = "buildkit.service"
	return disableService(ctx, name)
}
