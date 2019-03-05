package config

import (
	"encoding/json"
)

type CNI struct {
	Image         string `toml:"image" json:"-"`
	Version       string `toml:"-" json:"cniVersion,omitempty"`
	NetworkName   string `toml:"name" json:"name"`
	Type          string `toml:"type" json:"type"`
	Master        string `toml:"master" json:"master,omitempty"`
	IPAM          IPAM   `toml:"ipam" json:"ipam"`
	Bridge        string `toml:"bridge" json:"bridge,omitempty"`
	BridgeAddress string `toml:"bridge_address" json:"-"`
}

func (c *CNI) Bytes() []byte {
	data, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	return data
}

type IPAM struct {
	Type   string `toml:"type" json:"type"`
	Subnet string `toml:"subnet" json:"subnet"`
}

func (s *CNI) Name() string {
	return "cni"
}
