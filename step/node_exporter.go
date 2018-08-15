package step

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/crosbymichael/boss/config"
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
	Config *config.Config
}

func (s *NodeExporter) Name() string {
	return "node_exporter"
}

func (s *NodeExporter) Run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	const name = "nodeexporter.service"
	if err := install(ctx, client, s.Config.NodeMetrics.Image, clix); err != nil {
		return err
	}
	if err := writeUnit(name, metricsUnit); err != nil {
		return err
	}
	return startNewService(ctx, name)
}

func (s *NodeExporter) Remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	if err := client.ImageService().Delete(ctx, s.Config.NodeMetrics.Image); err != nil {
		return err
	}
	const name = "nodeexporter.service"
	return disableService(ctx, name)
}
