package config

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/urfave/cli"
)

const dhcpUnit = `[Unit]
Description=cni dhcp server
After=network.target

[Service]
ExecStartPre=/bin/rm -f /run/cni/dhcp.sock
ExecStart=/opt/containerd/bin/dhcp daemon
Restart=always

[Install]
WantedBy=multi-user.target`

type DHCP struct {
}

func (s *DHCP) Name() string {
	return "dhcp"
}

func (s *DHCP) Run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	const name = "cni-dhcp.service"
	if err := writeUnit(name, dhcpUnit); err != nil {
		return err
	}
	return startNewService(ctx, name)
}

func (s *DHCP) Remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	const name = "cni-dhcp.service"
	return disableService(ctx, name)
}
