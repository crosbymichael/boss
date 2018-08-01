package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cmd/ctr/commands/content"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/contrib/apparmor"
	"github.com/containerd/containerd/contrib/nvidia"
	"github.com/containerd/containerd/contrib/seccomp"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/platforms"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli"
)

var createCommand = cli.Command{
	Name:  "create",
	Usage: "create a container",
	Flags: []cli.Flag{
		cli.StringSliceFlag{
			Name:  "platform",
			Usage: "pull content from a specific platform",
			Value: &cli.StringSlice{platforms.Default()},
		},
		cli.BoolFlag{
			Name:  "all-platforms",
			Usage: "pull content from all platforms",
		},
	},
	Action: func(clix *cli.Context) error {
		var config Config
		if _, err := toml.DecodeFile(clix.Args().First(), &config); err != nil {
			return err
		}
		ctx := namespaces.WithNamespace(context.Background(), clix.GlobalString("namespace"))
		client, err := containerd.New(
			defaults.DefaultAddress,
			containerd.WithDefaultRuntime("io.containerd.runc.v1"),
		)
		if err != nil {
			return err
		}
		defer client.Close()
		image, err := getImage(ctx, client, config.Image, clix)
		if err != nil {
			return err
		}
		opts := []oci.SpecOpts{
			oci.WithImageConfigArgs(image, config.Args),
			oci.WithHostLocaltime,
			oci.WithNoNewPrivileges,
			apparmor.WithDefaultProfile("boss"),
			seccomp.WithDefaultProfile(),
			oci.WithEnv(config.Env),
			withMounts(config.Mounts),
		}
		if config.HostNetwork {
			opts = append(opts, oci.WithHostHostsFile, oci.WithHostResolvconf, oci.WithHostNamespace(specs.NetworkNamespace))
		} else {
			opts = append(opts, withBossResolvconf, withContainerHostsFile)
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
		flux := newFlux(client)
		_, err = client.NewContainer(
			ctx,
			config.ID,
			containerd.WithNewSpec(opts...),
			containerd.WithContainerLabels(toStrings(config.Labels)),
			withStatus(containerd.Running),
			WithNewSnapshotFromImage(flux, image),
			containerd.WithContainerExtension(configExtention, &config),
		)
		return err
	},
}

func withStatus(status containerd.ProcessStatus) func(context.Context, *containerd.Client, *containers.Container) error {
	return func(_ context.Context, _ *containerd.Client, c *containers.Container) error {
		ensureLabels(c)
		c.Labels[statusLabel] = string(status)
		return nil
	}
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

func ensureLabels(c *containers.Container) {
	if c.Labels == nil {
		c.Labels = make(map[string]string)
	}
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

func getImage(ctx context.Context, client *containerd.Client, ref string, clix *cli.Context) (containerd.Image, error) {
	image, err := client.GetImage(ctx, ref)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return nil, err
		}
		if image, err = content.Fetch(ctx, client, ref, clix); err != nil {
			return nil, err
		}
		if err := image.Unpack(ctx, containerd.DefaultSnapshotter); err != nil {
			return nil, err
		}
	}
	return image, nil
}

func withContainerHostsFile(ctx context.Context, _ oci.Client, c *containers.Container, s *oci.Spec) error {
	id := c.ID
	if err := os.MkdirAll(filepath.Join(rootDir, id), 0711); err != nil {
		return err
	}
	hostname := s.Hostname
	if hostname == "" {
		hostname = id
	}
	path := filepath.Join(rootDir, id, "hosts")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
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
		Source:      filepath.Join(rootDir, "resolv.conf"),
		Options:     []string{"rbind", "ro"},
	})
	return nil
}
