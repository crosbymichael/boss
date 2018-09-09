package config

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/urfave/cli"
)

const metricsUnit = `[Unit]
Description=prometheus node metrics

[Service]
ExecStart=/opt/containerd/bin/nodeexporter
Restart=always

[Install]
WantedBy=multi-user.target`

type NodeExporter struct {
	Image string `toml:"image"`
}

func (s *NodeExporter) Name() string {
	return "node-exporter"
}

func (s *NodeExporter) Run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	const name = "nodeexporter.service"
	if err := install(ctx, client, s.Image, clix); err != nil {
		return err
	}
	if err := writeUnit(name, metricsUnit); err != nil {
		return err
	}
	return startNewService(ctx, name)
}

func (s *NodeExporter) Remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	if err := client.ImageService().Delete(ctx, s.Image); err != nil {
		return err
	}
	const name = "nodeexporter.service"
	return disableService(ctx, name)
}
