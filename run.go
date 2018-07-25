package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cmd/ctr/commands/content"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/contrib/apparmor"
	"github.com/containerd/containerd/contrib/seccomp"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/runtime/restart"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli"
)

var runCommand = cli.Command{
	Name:  "run",
	Usage: "run a container",
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
		image, err := content.Fetch(ctx, client, config.Image, clix)
		if err != nil {
			return err
		}
		fmt.Printf("unpacking image into %s\n", containerd.DefaultSnapshotter)
		if err := image.Unpack(ctx, containerd.DefaultSnapshotter); err != nil {
			return err
		}
		opts := []oci.SpecOpts{
			oci.WithImageConfig(image),
			oci.WithHostLocaltime,
			oci.WithNoNewPrivileges,
			apparmor.WithDefaultProfile("boss"),
			seccomp.WithDefaultProfile(),
		}
		if config.Network.Host {
			opts = append(opts, oci.WithHostHostsFile, oci.WithHostResolvconf, oci.WithHostNamespace(specs.NetworkNamespace))
		}
		if config.Resources != nil {
			opts = append(opts, withResources(config.Resources))
		}
		for _, cm := range config.Mounts {
			opts = append(opts, oci.WithMounts([]specs.Mount{
				{
					Type:        cm.Type,
					Source:      cm.Source,
					Destination: cm.Destination,
					Options:     cm.Options,
				},
			}),
			)
		}
		logpath := filepath.Join(clix.GlobalString("log-path"), config.ID)
		f, err := os.Create(logpath)
		if err != nil {
			return err
		}
		f.Close()
		_, err = client.NewContainer(
			ctx,
			config.ID,
			containerd.WithNewSpec(opts...),
			restart.WithStatus(containerd.Running),
			restart.WithLogPath(logpath),
			containerd.WithNewSnapshot(config.ID, image),
		)
		return err
	},
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
