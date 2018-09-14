package config

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/urfave/cli"
)

const agentUnit = `[Unit]
Description=boss agent
After=containerd.service network.target

[Service]
ExecStartPre=/bin/mount -a
ExecStart=/usr/local/bin/boss agent
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target`

type Agent struct {
	PlainRemotes     []string `toml:"plain_remotes"`
	VolumeRoot       string   `toml:"volume_root"`
	AdvertiseAddress string   `toml:"advertise_address"`
	Peers            []string `toml:"peers"`
}

func (s *Agent) Name() string {
	return "agent"
}

func (s *Agent) Run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	const name = "boss-agent.service"
	if err := writeUnit(name, agentUnit); err != nil {
		return err
	}
	return startNewService(ctx, name)
}

func (s *Agent) Remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	const name = "boss-agent.service"
	return disableService(ctx, name)
}
