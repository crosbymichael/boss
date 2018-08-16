package config

import (
	"context"
	"encoding/json"

	"github.com/containerd/containerd"
	"github.com/urfave/cli"
)

type CNI struct {
	Image         string `toml:"image" json:"-"`
	Version       string `toml:"-" json:"cniVersion,omitempty"`
	NetworkName   string `toml:"name" json:"name"`
	Type          string `toml:"type" json:"type"`
	Master        string `toml:"master" json:"master,omitempty"`
	IPAM          IPAM   `toml:"ipam" json:"ipam"`
	BridgeAddress string `toml:"bridge_address" json:"-"`
}

func (c *CNI) SubSteps() (o []Step) {
	if c.IPAM.Type == "dhcp" {
		o = append(o, &DHCP{})
	}
	return o
}

func (c *CNI) Bytes() []byte {
	data, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	return data
}

type IPAM struct {
	Type string `toml:"type" json:"type"`
}

func (s *CNI) Name() string {
	return "cni"
}

func (s *CNI) Run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	return install(ctx, client, s.Image, clix)
}

func (s *CNI) Remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	return client.ImageService().Delete(ctx, s.Image)
}
