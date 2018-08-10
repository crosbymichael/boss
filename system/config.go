package system

import (
	"context"
	"net"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/namespaces"
	gocni "github.com/containerd/go-cni"
	"github.com/crosbymichael/boss/config"
	"github.com/hashicorp/consul/api"
	"github.com/pkg/errors"
)

// Register is an object that registers and manages service information in its backend
type Register interface {
	Register(id, name, ip string, s config.Service) error
	Deregister(id string) error
	EnableMaintainance(id, msg string) error
	DisableMaintainance(id string) error
}

type Network interface {
	Create(containerd.Container) (string, error)
	Remove(containerd.Container) error
}

// Load the system config from disk
// fix up any missing fields with runtime data
func Load() (*config.Config, error) {
	var c config.Config
	if _, err := toml.DecodeFile(config.Path, &c); err != nil {
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

// Context returns a new context to be used by boss
func Context() context.Context {
	return namespaces.WithNamespace(context.Background(), config.DefaultNamespace)
}

// NewClient returns a new containerd client
func NewClient() (*containerd.Client, error) {
	return containerd.New(
		defaults.DefaultAddress,
		containerd.WithDefaultRuntime(config.DefaultRuntime),
	)
}

// GetNetwork returns a network for the givin name
func GetNetwork(c *config.Config, name string) (Network, error) {
	ip, err := GetIP(c.Iface)
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
		n, err := gocni.New(gocni.WithPluginDir([]string{"/opt/containerd/bin"}), gocni.WithConf(c.CNI.Bytes()), gocni.WithLoNetwork)
		if err != nil {
			return nil, err
		}
		return &cni{network: n}, nil
	}
	return nil, errors.Errorf("network %s does not exist", name)
}

func GetRegister(c *config.Config) (Register, error) {
	if c.Consul != nil {
		consul, err := api.NewClient(api.DefaultConfig())
		if err != nil {
			return nil, err
		}
		return &Consul{
			client: consul,
		}, nil
	}
	return &nullRegister{}, nil
}

func GetNameservers(c *config.Config) ([]string, error) {
	if c.Consul != nil {
		consul, err := api.NewClient(api.DefaultConfig())
		if err != nil {
			return nil, err
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

func Ready(c *config.Config) error {
	return nil
}

var ErrIPAddressNotFound = errors.New("box: ip address for interface not found")

func GetIP(name string) (string, error) {
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
	return "", ErrIPAddressNotFound
}

func ipv4(n *net.IPNet) string {
	if n.IP.To4() == nil {
		return ""
	}
	return n.IP.To4().String()
}
