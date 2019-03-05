package config

import (
	"context"
	"os"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/containerd/containerd"
	gocni "github.com/containerd/go-cni"
	v1 "github.com/crosbymichael/boss/api/v1"
	"github.com/crosbymichael/boss/cni"
	"github.com/crosbymichael/boss/consulregister"
	"github.com/crosbymichael/boss/util"
	"github.com/hashicorp/consul/api"
	"github.com/pkg/errors"
)

const (
	Path = "/etc/boss/boss.toml"
)

type ConfigStore interface {
	Write(context.Context, *v1.Container) error
	Watch(context.Context, containerd.Container, *v1.Container) (<-chan error, error)
}

// Load the system config from disk
// fix up any missing fields with runtime data
func Load() (*Config, error) {
	var c Config
	if _, err := toml.DecodeFile(Path, &c); err != nil {
		return nil, err
	}
	if c.ID == "" {
		id, err := os.Hostname()
		if err != nil {
			return nil, err
		}
		c.ID = id
	}
	if c.Iface == "" {
		c.Iface = "eth0"
	}
	return &c, nil
}

var (
	consul     *api.Client
	consulErr  error
	consulOnce sync.Once
)

func getConsul() {
	consul, consulErr = api.NewClient(api.DefaultConfig())
}

type Config struct {
	ID           string        `toml:"id"`
	Iface        string        `toml:"iface"`
	Domain       string        `toml:"domain"`
	Buildkit     *Buildkit     `toml:"buildkit"`
	CNI          *CNI          `toml:"cni"`
	Consul       *Consul       `toml:"consul"`
	NodeExporter *NodeExporter `toml:"nodeexporter"`
	Nameservers  []string      `toml:"nameservers"`
	Timezone     string        `toml:"timezone"`
	MOTD         *MOTD         `toml:"motd"`
	SSH          *SSH          `toml:"ssh"`
	Agent        Agent         `toml:"agent"`
	Containerd   Containerd    `toml:"containerd"`
	Criu         *Criu         `toml:"criu"`
}

func (c *Config) Store() (ConfigStore, error) {
	if c.Consul != nil {
		consulOnce.Do(getConsul)
		if consulErr != nil {
			return nil, consulErr
		}
		return &configStore{
			consul: consul,
		}, nil
	}
	return &nullStore{}, nil
}

// GetNetwork returns a network for the givin name
func (c *Config) GetNetwork(name string) (v1.Network, error) {
	ip, err := util.GetIP(c.Iface)
	if err != nil {
		return nil, err
	}
	switch name {
	case "", "none":
		return &none{}, nil
	case "host":
		return &host{ip: ip}, nil
	case "cni":
		if c.CNI == nil {
			return nil, errors.New("[cni] is not enabled in the system config")
		}
		if c.CNI.Type == "macvlan" && c.CNI.BridgeAddress == "" {
			return nil, errors.New("bridge_address must be specified with macvlan")
		}
		// populate cni data from main config if fields are missing
		c.CNI.Version = "0.3.1"
		if c.CNI.NetworkName == "" {
			c.CNI.NetworkName = c.Domain
		}
		if c.CNI.Master == "" {
			c.CNI.Master = c.Iface
		}
		n, err := gocni.New(
			gocni.WithPluginDir([]string{"/opt/containerd/bin"}),
			gocni.WithConf(c.CNI.Bytes()),
			gocni.WithLoNetwork,
		)
		if err != nil {
			return nil, err
		}
		return cni.New(c.CNI.Type, c.Iface, c.CNI.BridgeAddress, n)
	}
	return nil, errors.Errorf("network %s does not exist", name)
}

func (c *Config) GetNameservers() ([]string, error) {
	if c.Consul != nil {
		consulOnce.Do(getConsul)
		if consulErr != nil {
			return nil, consulErr
		}
		nodes, _, err := consul.Catalog().Nodes(&api.QueryOptions{})
		if err != nil {
			return nil, err
		}
		var ns []string
		for _, n := range nodes {
			ns = append(ns, n.Address)
		}
		return ns, nil
	}
	if len(c.Nameservers) == 0 {
		return []string{
			"8.8.8.8",
			"8.8.4.4",
		}, nil
	}
	return c.Nameservers, nil
}

func (c *Config) GetRegister() (v1.Register, error) {
	if c.Consul != nil {
		consulOnce.Do(getConsul)
		if consulErr != nil {
			return nil, consulErr
		}
		return consulregister.New(consul), nil
	}
	return &nullRegister{}, nil
}

func (c *Config) consul() bool {
	return c.Consul != nil
}
