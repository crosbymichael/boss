package system

import (
	"context"
	"encoding/json"
	"net"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/namespaces"
	gocni "github.com/containerd/go-cni"
	"github.com/crosbymichael/boss/config"
	"github.com/hashicorp/consul/api"
)

const (
	DefaultRuntime   = "io.containerd.runc.v1"
	DefaultNamespace = "boss"
)

// Register is an object that registers and manages service information in its backend
type Register interface {
	Register(id, name, ip string, s config.Service) error
	Deregister(id string) error
	EnableMaintainance(id, msg string) error
	DisableMaintainance(id string) error
}

type Network interface {
	Create(containerd.Task) (string, error)
	Remove(containerd.Container) error
}

func Load(path string) (*Config, error) {
	var c Config
	if _, err := toml.DecodeFile(path, &c); err != nil {
		return nil, err
	}
	if c.Namespace == "" {
		c.Namespace = DefaultNamespace
	}
	if c.Runtime == "" {
		c.Runtime = DefaultRuntime
	}
	if c.ID == "" {
		id, err := os.Hostname()
		if err != nil {
			return nil, err
		}
		c.ID = id
	}
	c.context = namespaces.WithNamespace(context.Background(), c.Namespace)
	if c.Agent.Interval == 0 {
		c.Agent.Interval = 10
	}
	if len(c.Nameservers) == 0 {
		c.Nameservers = []string{
			"8.8.8.8",
			"8.8.4.4",
		}
	}
	return &c, nil
}

func Ready(c *Config) error {
	c.networks = make(map[string]Network)
	c.networks["host"] = &host{}
	c.networks["none"] = &none{}
	c.networks[""] = &none{}
	if c.CNI != nil {
		n, err := gocni.New(gocni.WithPluginDir([]string{"/opt/containerd/bin"}), gocni.WithConf(c.CNI.Bytes()))
		if err != nil {
			return err
		}
		c.networks["cni"] = &cni{
			network: n,
		}
	}
	client, err := containerd.New(
		defaults.DefaultAddress,
		containerd.WithDefaultRuntime(c.Runtime),
	)
	if err != nil {
		return err
	}
	c.client = client
	if c.Register == "consul" {
		consul, err := api.NewClient(api.DefaultConfig())
		if err != nil {
			return err
		}
		c.consul = consul
		nodes, _, err := consul.Catalog().Nodes(&api.QueryOptions{})
		if err != nil {
			return err
		}
		c.Nameservers = nil
		for _, n := range nodes {
			host, _, err := net.SplitHostPort(n.Address)
			if err != nil {
				return err
			}
			c.Nameservers = append(c.Nameservers, host)
		}

	}
	return nil
}

type Config struct {
	ID          string      `toml:"id"`
	Domain      string      `toml:"domain"`
	Namespace   string      `toml:"namespace"`
	Register    string      `toml:"register"`
	Debug       bool        `toml:"debug"`
	Runtime     string      `toml:"runtime"`
	Agent       Agent       `toml:"agent"`
	SSH         SSH         `toml:"ssh"`
	Buildkit    Buildkit    `toml:"buildkit"`
	Containerd  Containerd  `toml:"containerd"`
	CNI         *CNI        `toml:"cni"`
	NodeMetrics NodeMetrics `toml:"nodemetrics"`
	Nameservers []string    `toml:"nameservers"`

	networks map[string]Network
	context  context.Context
	client   *containerd.Client
	consul   *api.Client
}

func (c *Config) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

func (c *Config) Context() context.Context {
	return c.context
}

func (c *Config) Client() *containerd.Client {
	return c.client
}

func (c *Config) Consul() *api.Client {
	return c.consul
}

func (c *Config) GetRegister() Register {
	if c.consul == nil {
		return &nullRegister{}
	}
	return &Consul{
		client: c.consul,
	}
}

func (c *Config) Network(id string) Network {
	return c.networks[id]
}

type Agent struct {
	Interval int `toml:"interval"`
}

type SSH struct {
	Admin          string `toml:"admin"`
	AuthorizedKeys string `toml:"authorized_keys"`
}

type Buildkit struct {
	Image   string `toml:"image"`
	Enabled bool   `toml:"enabled"`
}

type Containerd struct {
	Level          string   `toml:"level"`
	Disable        []string `toml:"disable"`
	MetricsAddress string   `toml:"metrics_address"`
}

type CNI struct {
	Version string `toml:"version" json:"cniVersion,omitempty"`
	Image   string `toml:"image" json:"-"`
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
