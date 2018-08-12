package main

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/crosbymichael/boss/config"
	"github.com/urfave/cli"
)

type cniStep struct {
	config *config.Config
}

func (s *cniStep) name() string {
	return "cni"
}

func (s *cniStep) run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	return install(ctx, client, s.config.CNI.Image, clix)
}

func (s *cniStep) remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	if err := client.ImageService().Delete(ctx, s.config.CNI.Image); err != nil {
		return err
	}
	return nil
}

const dhcpUnit = `[Unit]
Description=cni dhcp server
After=network.target

[Service]
ExecStartPre=/bin/rm -f /run/cni/dhcp.sock
ExecStart=/opt/containerd/bin/dhcp daemon
Restart=always

[Install]
WantedBy=multi-user.target`

type dhcpStep struct {
}

func (s *dhcpStep) name() string {
	return "dhcp"
}

func (s *dhcpStep) run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	const name = "cni-dhcp.service"
	if err := writeUnit(name, dhcpUnit); err != nil {
		return err
	}
	return startNewService(ctx, name)
}

func (s *dhcpStep) remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	const name = "cni-dhcp.service"
	return disableService(ctx, name)
}
