package main

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

type nodeMetricsStep struct {
	config *config.Config
}

func (s *nodeMetricsStep) name() string {
	return "node exporter"
}

func (s *nodeMetricsStep) run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	const name = "nodeexporter.service"
	if err := install(ctx, client, s.config.NodeMetrics.Image, clix); err != nil {
		return err
	}
	if err := writeUnit(name, metricsUnit); err != nil {
		return err
	}
	return startNewService(ctx, name)
}

func (s *nodeMetricsStep) remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	if err := client.ImageService().Delete(ctx, s.config.NodeMetrics.Image); err != nil {
		return err
	}
	const name = "nodeexporter.service"
	return disableService(ctx, name)
}
