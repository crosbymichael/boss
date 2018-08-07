package system

import (
	"context"
	"encoding/json"
	"errors"
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
	if os.Geteuid() == 0 {
		client, err := containerd.New(
			defaults.DefaultAddress,
			containerd.WithDefaultRuntime(c.Runtime),
		)
		if err != nil {
			return nil, err
		}
		c.client = client
	}
	if c.Iface == "" {
		c.Iface = "eth0"
	}
	ip, err := getIP(c.Iface)
	if err != nil {
		return nil, err
	}
	c.ip = ip

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
	if c.Consul != nil {
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
			c.Nameservers = append(c.Nameservers, n.Address)
		}
	}
	return nil
}

type Config struct {
	ID          string        `toml:"id"`
	Iface       string        `toml:"iface"`
	Domain      string        `toml:"domain"`
	Namespace   string        `toml:"namespace"`
	Debug       bool          `toml:"debug"`
	Runtime     string        `toml:"runtime"`
	Agent       Agent         `toml:"agent"`
	Buildkit    *Buildkit     `toml:"buildkit"`
	CNI         *CNI          `toml:"cni"`
	Consul      *ConsulConfig `toml:"consul"`
	NodeMetrics *NodeMetrics  `toml:"nodemetrics"`
	Nameservers []string      `toml:"nameservers"`
	//SSH         SSH          `toml:"ssh"`

	networks map[string]Network
	context  context.Context
	client   *containerd.Client
	consul   *api.Client
	ip       string
}

func (c *Config) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

func (c *Config) IP() string {
	return c.ip
}

func (c *Config) Context() context.Context {
	return c.context
}

func (c *Config) Client() *containerd.Client {
	return c.client
}

func (c *Config) GetConsul() *api.Client {
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

type ConsulConfig struct {
	Image     string `toml:"image"`
	Bootstrap bool   `toml:"bootstrap"`
}

type Agent struct {
	Interval int `toml:"interval"`
}

type SSH struct {
	Admin          string `toml:"admin"`
	AuthorizedKeys string `toml:"authorized_keys"`
}

type Buildkit struct {
	Image string `toml:"image"`
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

var errIPAddressNotFound = errors.New("box: ip address for interface not found")

func getIP(name string) (string, error) {
	i, err := net.InterfaceByName(name)
	if err != nil {
		return "", err
	}
	return getIPf(i, ipv4)
}

func getIPf(i *net.Interface, ipfunc func(n *net.IPNet) string) (string, error) {
	addrs, err := i.Addrs()
	if err != nil {
		return "", err
	}
	for _, a := range addrs {
		n, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		s := ipfunc(n)
		if s == "" {
			continue
		}
		return s, nil
	}
	return "", errIPAddressNotFound
}

func ipv4(n *net.IPNet) string {
	if n.IP.To4() == nil {
		return ""
	}
	return n.IP.To4().String()
}
