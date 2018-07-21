package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/cmd/ctr/commands/content"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/contrib/apparmor"
	"github.com/containerd/containerd/contrib/seccomp"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	cni "github.com/containerd/go-cni"
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
		container, err := client.NewContainer(
			ctx,
			config.ID,
			containerd.WithNewSpec(opts...),
			containerd.WithContainerLabels(map[string]string{
				"io.containerd/restart.status":  "running",
				"io.containerd/restart.logpath": logpath,
			}),
			containerd.WithNewSnapshot(config.ID, image),
		)
		if err != nil {
			return err
		}
		fmt.Printf("created container %s with logpath %s\n", config.ID, logpath)
		task, err := container.NewTask(ctx, cio.NullIO)
		if err != nil {
			container.Delete(ctx, containerd.WithSnapshotCleanup)
			return err
		}
		if config.Network.CNI {
			networking, err := cni.New()
			if err != nil {
				return err
			}
			fmt.Println("using CNI networking...")
			result, err := networking.Setup(config.ID, fmt.Sprintf("/proc/%d/ns/net", task.Pid()))
			if err != nil {
				task.Delete(ctx, containerd.WithProcessKill)
				container.Delete(ctx, containerd.WithSnapshotCleanup)
				return err
			}
			fmt.Printf("setup networking for %s with IP %#v\n", config.ID, result)
		}

		fmt.Println("starting container...")
		if err := task.Start(ctx); err != nil {
			task.Delete(ctx, containerd.WithProcessKill)
			container.Delete(ctx, containerd.WithSnapshotCleanup)
			return err
		}
		fmt.Printf("container %s started, have a great day!\n", config.ID)
		return nil
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
