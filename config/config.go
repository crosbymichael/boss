package config

import (
	"context"

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
	ID           string        `toml:"id"`
	Iface        string        `toml:"iface"`
	Domain       string        `toml:"domain"`
	Buildkit     *Buildkit     `toml:"buildkit"`
	CNI          *CNI          `toml:"cni"`
	Consul       *Consul       `toml:"consul"`
	NodeExporter *NodeExporter `toml:"node_exporter"`
	Nameservers  []string      `toml:"nameservers"`
	Timezone     string        `toml:"timezone"`
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

type SSH struct {
	Admin          string `toml:"admin"`
	AuthorizedKeys string `toml:"authorized_keys"`
}
