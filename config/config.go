package config

import (
	"context"
	"os"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/containerd/containerd"
	"github.com/crosbymichael/boss/api/v1"
	"github.com/hashicorp/consul/api"
)

const Path = "/etc/boss/boss.toml"

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
	NodeExporter *NodeExporter `toml:"node_exporter"`
	Nameservers  []string      `toml:"nameservers"`
	Timezone     string        `toml:"timezone"`
	MOTD         *MOTD         `toml:"motd"`
	SSH          *SSH          `toml:"ssh"`
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

func (c *Config) Steps() []Step {
	steps := []Step{
		&Mkdir{},
		&Systemd{},
		&Timezone{TZ: c.Timezone},
	}
	if c.consul() {
		// set the config for other things
		c.Consul.c = c
		steps = append(steps, c.Consul)
		steps = append(steps, c.Consul.SubSteps()...)
	}
	if c.NodeExporter != nil {
		steps = append(steps, c.NodeExporter)
		if c.consul() {
			steps = append(steps, &RegisterService{
				Config: c,
				ID:     "node-exporter",
				Tags: []string{
					"metrics",
				},
				Port: 9100,
			})
		}
	}
	if c.Buildkit != nil {
		steps = append(steps, c.Buildkit)
		if c.consul() {
			steps = append(steps, &RegisterService{
				Config: c,
				ID:     "buildkit",
				Port:   9500,
				Tags:   []string{"build"},
			})
		}
	}
	if c.CNI != nil {
		steps = append(steps, c.CNI)
		steps = append(steps, c.CNI.SubSteps()...)
	}
	if c.MOTD != nil {
		steps = append(steps, c.MOTD)
	}
	if c.SSH != nil {
		steps = append(steps, c.SSH)
	}
	if c.consul() {
		// add DNS at the end so we can still pull images in the other steps
		steps = append(steps, &DNS{
			ID: c.ID,
		})
	}
	return steps
}

func (c *Config) consul() bool {
	return c.Consul != nil
}
