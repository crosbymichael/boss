package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/contrib/apparmor"
	"github.com/containerd/containerd/contrib/nvidia"
	"github.com/containerd/containerd/contrib/seccomp"
	"github.com/containerd/containerd/oci"
	gocni "github.com/containerd/go-cni"
	"github.com/containerd/typeurl"
	"github.com/crosbymichael/boss/api/v1"
	"github.com/crosbymichael/boss/cni"
	"github.com/crosbymichael/boss/consulregister"
	"github.com/crosbymichael/boss/util"
	"github.com/gogo/protobuf/types"
	"github.com/hashicorp/consul/api"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

const (
	Path          = "/etc/boss/boss.toml"
	CurrentConfig = "io.boss/container"
	LastConfig    = "io.boss/container.last"
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
	NodeExporter *NodeExporter `toml:"node_exporter"`
	Nameservers  []string      `toml:"nameservers"`
	Timezone     string        `toml:"timezone"`
	MOTD         *MOTD         `toml:"motd"`
	SSH          *SSH          `toml:"ssh"`
	Agent        Agent         `toml:"agent"`
	Containerd   Containerd    `toml:"containerd"`
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
		n, err := gocni.New(gocni.WithPluginDir([]string{"/opt/containerd/bin"}), gocni.WithConf(c.CNI.Bytes()), gocni.WithLoNetwork)
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

func (c *Config) Steps() []Step {
	steps := []Step{
		&Mkdir{},
		&Systemd{},
		&Timezone{TZ: c.Timezone},
		&c.Agent,
	}
	if c.consul() {
		// set the config for other things
		c.Consul.c = c
		steps = append(steps, c.Consul)
		steps = append(steps, c.Consul.SubSteps()...)
		steps = append(steps, &RegisterService{
			Config: c,
			ID:     "agent",
			Tags: []string{
				"boss",
			},
			Port: 1337,
			Check: &v1.HealthCheck{
				Type: "grpc",
			},
		})
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

// WithBossConfig is a containerd.NewContainerOpts for spec and container configuration
func WithBossConfig(config *v1.Container, image containerd.Image) func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
	return func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
		// generate the spec
		if err := containerd.WithNewSpec(specOpt(config, image))(ctx, client, c); err != nil {
			return err
		}
		// save the config as a container extension
		return containerd.WithContainerExtension(CurrentConfig, config)(ctx, client, c)
	}
}

func WithSetPreviousConfig(ctx context.Context, client *containerd.Client, c *containers.Container) error {
	c.Extensions[LastConfig] = c.Extensions[CurrentConfig]
	return nil
}

func WithRollback(ctx context.Context, client *containerd.Client, c *containers.Container) error {
	d := c.Extensions[LastConfig]
	if d.Value == nil {
		return nil
	}
	c.Extensions[CurrentConfig] = d
	return nil
}

func specOpt(config *v1.Container, image containerd.Image) oci.SpecOpts {
	opts := []oci.SpecOpts{
		oci.WithImageConfigArgs(image, config.Process.Args),
		oci.WithHostLocaltime,
		oci.WithNoNewPrivileges,
		apparmor.WithDefaultProfile("boss"),
		seccomp.WithDefaultProfile(),
		oci.WithEnv(config.Process.Env),
		withMounts(config.Mounts),
		withConfigs(config.Configs),
	}
	if config.Network == "host" {
		opts = append(opts, oci.WithHostHostsFile, oci.WithHostResolvconf, oci.WithHostNamespace(specs.NetworkNamespace))
	} else if config.Network == "cni" {
		opts = append(opts, withBossResolvconf, withContainerHostsFile, oci.WithLinuxNamespace(specs.LinuxNamespace{
			Type: specs.NetworkNamespace,
			Path: v1.NetworkPath(config.ID),
		}),
			oci.WithHostname(config.ID),
		)
	}
	if config.Resources != nil {
		opts = append(opts, withResources(config.Resources))
	}
	if config.Gpus != nil {
		opts = append(opts, nvidia.WithGPUs(
			nvidia.WithDevices(ints(config.Gpus.Devices)...),
			nvidia.WithCapabilities(toGpuCaps(config.Gpus.Capabilities)...),
		),
		)
	}
	if config.Process.User != nil {
		opts = append(opts, oci.WithUIDGID(config.Process.User.Uid, config.Process.User.Gid))
	}
	return oci.Compose(opts...)
}

func ints(i []int64) (o []int) {
	for _, ii := range i {
		o = append(o, int(ii))
	}
	return o
}

func toStrings(ss []string) map[string]string {
	m := make(map[string]string, len(ss))
	for _, s := range ss {
		parts := strings.SplitN(s, "=", 2)
		m[parts[0]] = parts[1]
	}
	return m
}

func toGpuCaps(ss []string) (o []nvidia.Capability) {
	for _, s := range ss {
		o = append(o, nvidia.Capability(s))
	}
	return o
}

func withResources(r *v1.Resources) oci.SpecOpts {
	return func(ctx context.Context, _ oci.Client, c *containers.Container, s *oci.Spec) error {
		if r.Memory > 0 {
			limit := r.Memory * 1024 * 1024
			s.Linux.Resources.Memory = &specs.LinuxMemory{
				Limit: &limit,
			}
		}
		if r.Cpus > 0 {
			period := uint64(100000)
			quota := int64(r.Cpus * 100000.0)
			s.Linux.Resources.CPU = &specs.LinuxCPU{
				Quota:  &quota,
				Period: &period,
			}
		}
		if r.Score != 0 {
			score := int(r.Score)
			s.Process.OOMScoreAdj = &score
		}
		if r.NoFile > 0 {
			s.Process.Rlimits = []specs.POSIXRlimit{
				{
					Type: "RLIMIT_NOFILE",
					Hard: r.NoFile,
					Soft: r.NoFile,
				},
			}
		}
		return nil
	}
}

func withMounts(mounts []*v1.Mount) oci.SpecOpts {
	return func(ctx context.Context, _ oci.Client, c *containers.Container, s *oci.Spec) error {
		for _, cm := range mounts {
			if cm.Type == "bind" {
				// create source if it does not exist
				if err := os.MkdirAll(filepath.Dir(cm.Source), 0755); err != nil {
					return err
				}
				if err := os.Mkdir(cm.Source, 0755); err != nil {
					if !os.IsExist(err) {
						return err
					}
				} else {
					if err := os.Chown(cm.Source, int(s.Process.User.UID), int(s.Process.User.GID)); err != nil {
						return err
					}
				}
			}
			s.Mounts = append(s.Mounts, specs.Mount{
				Type:        cm.Type,
				Source:      cm.Source,
				Destination: cm.Destination,
				Options:     cm.Options,
			})
		}
		return nil
	}
}

func withConfigs(files map[string]*v1.Config) oci.SpecOpts {
	return func(ctx context.Context, _ oci.Client, c *containers.Container, s *oci.Spec) error {
		for name, f := range files {
			s.Mounts = append(s.Mounts, specs.Mount{
				Type:        "bind",
				Source:      v1.ConfigPath(c.ID, name),
				Destination: f.Path,
				Options: []string{
					"ro", "rbind",
				},
			})
		}
		return nil
	}
}

func withContainerHostsFile(ctx context.Context, _ oci.Client, c *containers.Container, s *oci.Spec) error {
	id := c.ID
	if err := os.MkdirAll(filepath.Join(v1.Root, id), 0711); err != nil {
		return err
	}
	hostname := s.Hostname
	if hostname == "" {
		hostname = id
	}
	path := filepath.Join(v1.Root, id, "hosts")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := f.Chmod(0666); err != nil {
		return err
	}
	if _, err := f.WriteString("127.0.0.1       localhost\n"); err != nil {
		return err
	}
	if _, err := f.WriteString(fmt.Sprintf("127.0.0.1       %s\n", hostname)); err != nil {
		return err
	}
	if _, err := f.WriteString("::1     localhost ip6-localhost ip6-loopback\n"); err != nil {
		return err
	}
	s.Mounts = append(s.Mounts, specs.Mount{
		Destination: "/etc/hosts",
		Type:        "bind",
		Source:      path,
		Options:     []string{"rbind", "ro"},
	})
	return nil
}

func withBossResolvconf(ctx context.Context, _ oci.Client, c *containers.Container, s *oci.Spec) error {
	s.Mounts = append(s.Mounts, specs.Mount{
		Destination: "/etc/resolv.conf",
		Type:        "bind",
		Source:      filepath.Join(v1.Root, c.ID, "resolv.conf"),
		Options:     []string{"rbind", "ro"},
	})
	return nil
}

func GetConfig(ctx context.Context, container containerd.Container) (*v1.Container, error) {
	info, err := container.Info(ctx)
	if err != nil {
		return nil, err
	}
	d := info.Extensions[CurrentConfig]
	return UnmarshalConfig(&d)
}

var ErrOldConfigFormat = errors.New("old config format on container")

func UnmarshalConfig(any *types.Any) (*v1.Container, error) {
	v, err := typeurl.UnmarshalAny(any)
	if err != nil {
		return nil, err
	}
	c, ok := v.(*v1.Container)
	if !ok {
		return nil, ErrOldConfigFormat
	}
	return c, nil
}
