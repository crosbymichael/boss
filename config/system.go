package config

import (
	"context"
	"encoding/json"

	"github.com/containerd/containerd"
)

// Register is an object that registers and manages service information in its backend
type Register interface {
	Register(id, name, ip string, s Service) error
	Deregister(id, name string) error
	EnableMaintainance(id, name, msg string) error
	DisableMaintainance(id, name string) error
}

type Network interface {
	Create(context.Context, containerd.Container) (string, error)
	Remove(context.Context, containerd.Container) error
}

type ConfigStore interface {
	Write(context.Context, *Container) error
	Watch(context.Context, containerd.Container, *Container) (<-chan error, error)
}

const (
	DefaultRuntime   = "io.containerd.runc.v1"
	DefaultNamespace = "boss"
	Path             = "/etc/boss/boss.toml"
)

type Config struct {
	ID          string        `toml:"id"`
	Iface       string        `toml:"iface"`
	Domain      string        `toml:"domain"`
	Buildkit    *Buildkit     `toml:"buildkit"`
	CNI         *CNI          `toml:"cni"`
	Consul      *ConsulConfig `toml:"consul"`
	NodeMetrics *NodeMetrics  `toml:"nodemetrics"`
	Nameservers []string      `toml:"nameservers"`
	Timezone    string        `toml:"timezone"`
}

type ConsulConfig struct {
	Image string `toml:"image"`
}

type SSH struct {
	Admin          string `toml:"admin"`
	AuthorizedKeys string `toml:"authorized_keys"`
}

type Buildkit struct {
	Image string `toml:"image"`
}

type CNI struct {
	Image   string `toml:"image" json:"-"`
	Version string `toml:"-" json:"cniVersion,omitempty"`
	Name    string `toml:"name" json:"name"`
	Type    string `toml:"type" json:"type"`
	Master  string `toml:"master" json:"master,omitempty"`
	IPAM    IPAM   `toml:"ipam" json:"ipam"`
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

type NodeMetrics struct {
	Image string `toml:"image"`
}
