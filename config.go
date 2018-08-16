package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/box/box/config/v1"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/contrib/apparmor"
	"github.com/containerd/containerd/contrib/nvidia"
	"github.com/containerd/containerd/contrib/seccomp"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/typeurl"
	"github.com/crosbymichael/boss/config"
	"github.com/gogo/protobuf/types"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const (
	CurrentConfig = "io.boss/container"
)

type Container struct {
	ID        string             `toml:"id"`
	Image     string             `toml:"image"`
	Resources *Resources         `toml:"resources"`
	GPUs      *GPUs              `toml:"gpus"`
	Mounts    []Mount            `toml:"mounts"`
	Env       []string           `toml:"env"`
	Args      []string           `toml:"args"`
	UID       *int               `toml:"uid"`
	GID       *int               `toml:"gid"`
	Network   string             `toml:"network"`
	Services  map[string]Service `toml:"services"`
	Configs   map[string]File    `toml:"configs"`
}

func (c *Container) Proto() *v1.Container {
	panic("proto not done")
	return nil
}

type File struct {
	Path    string `toml:"path"`
	Source  string `toml:"source"`
	Content string `toml:"content"`
	// Signal to be sent when the config changes
	Signal string `toml:"signal"`
}

type Service struct {
	Port          int       `toml:"port"`
	Labels        []string  `toml:"labels"`
	URL           string    `toml:"url"`
	CheckType     CheckType `toml:"check_type"`
	CheckInterval int       `toml:"check_interval"`
	CheckTimeout  int       `toml:"check_timeout"`
	CheckMethod   string    `toml:"check_method"`
}

type CheckType string

const (
	HTTP CheckType = "http"
	TCP  CheckType = "tcp"
	GRPC CheckType = "grpc"
)

type Resources struct {
	CPU    float64 `toml:"cpu"`
	Memory int64   `toml:"memory"`
	Score  int     `toml:"score"`
	NoFile uint64  `toml:"no_file"`
}

type GPUs struct {
	Devices     []int    `toml:"devices"`
	Capbilities []string `toml:"capabilities"`
}

type Mount struct {
	Type        string   `toml:"type"`
	Source      string   `toml:"source"`
	Destination string   `toml:"destination"`
	Options     []string `toml:"options"`
}

// WithBossConfig is a containerd.NewContainerOpts for spec and container configuration
func WithBossConfig(config *Container, image containerd.Image) func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
	return func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
		// generate the spec
		if err := containerd.WithNewSpec(config.specOpt(image))(ctx, client, c); err != nil {
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

func (config *Container) specOpt(image containerd.Image) oci.SpecOpts {
	opts := []oci.SpecOpts{
		oci.WithImageConfigArgs(image, config.Args),
		oci.WithHostLocaltime,
		oci.WithNoNewPrivileges,
		apparmor.WithDefaultProfile("boss"),
		seccomp.WithDefaultProfile(),
		oci.WithEnv(config.Env),
		withMounts(config.Mounts),
		withConfigs(config.Configs),
	}
	if config.Network == "host" {
		opts = append(opts, oci.WithHostHostsFile, oci.WithHostResolvconf, oci.WithHostNamespace(specs.NetworkNamespace))
	} else if config.Network == "cni" {
		opts = append(opts, withBossResolvconf, withContainerHostsFile, oci.WithLinuxNamespace(specs.LinuxNamespace{
			Type: specs.NetworkNamespace,
			Path: NetworkPath(config.ID),
		}),
			oci.WithHostname(config.ID),
		)
	}
	if config.Resources != nil {
		opts = append(opts, withResources(config.Resources))
	}
	if config.GPUs != nil {
		opts = append(opts, nvidia.WithGPUs(
			nvidia.WithDevices(config.GPUs.Devices...),
			nvidia.WithCapabilities(toGpuCaps(config.GPUs.Capbilities)...),
		),
		)
	}
	if config.UID != nil {
		gid := 0
		if config.GID != nil {
			gid = *config.GID
		}
		opts = append(opts, oci.WithUIDGID(uint32(*config.UID), uint32(gid)))
	}
	return oci.Compose(opts...)
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

func withResources(r *Resources) oci.SpecOpts {
	return func(ctx context.Context, _ oci.Client, c *containers.Container, s *oci.Spec) error {
		if r.Memory > 0 {
			limit := r.Memory * 1024 * 1024
			s.Linux.Resources.Memory = &specs.LinuxMemory{
				Limit: &limit,
			}
		}
		if r.CPU > 0 {
			period := uint64(100000)
			quota := int64(r.CPU * 100000.0)
			s.Linux.Resources.CPU = &specs.LinuxCPU{
				Quota:  &quota,
				Period: &period,
			}
		}
		if r.Score != 0 {
			s.Process.OOMScoreAdj = &r.Score
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

func withMounts(mounts []Mount) oci.SpecOpts {
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

func withConfigs(files map[string]File) oci.SpecOpts {
	return func(ctx context.Context, _ oci.Client, c *containers.Container, s *oci.Spec) error {
		for name, f := range files {
			s.Mounts = append(s.Mounts, specs.Mount{
				Type:        "bind",
				Source:      ConfigPath(c.ID, name),
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
	if err := os.MkdirAll(filepath.Join(config.Root, id), 0711); err != nil {
		return err
	}
	hostname := s.Hostname
	if hostname == "" {
		hostname = id
	}
	path := filepath.Join(config.Root, id, "hosts")
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
		Source:      filepath.Join(config.Root, c.ID, "resolv.conf"),
		Options:     []string{"rbind", "ro"},
	})
	return nil
}

func GetConfig(ctx context.Context, container containerd.Container) (*Container, error) {
	info, err := container.Info(ctx)
	if err != nil {
		return nil, err
	}
	d := info.Extensions[CurrentConfig]
	return UnmarshalConfig(&d)
}

func UnmarshalConfig(any *types.Any) (*Container, error) {
	v, err := typeurl.UnmarshalAny(any)
	if err != nil {
		return nil, err
	}
	return v.(*Container), nil
}
