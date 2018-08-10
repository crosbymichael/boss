package main

import (
	"archive/tar"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/archive"
	"github.com/containerd/containerd/archive/compression"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/content/local"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/crosbymichael/boss/system"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli"
)

const containerdUnit = `[Unit]
Description=containerd container runtime
Documentation=https://containerd.io
After=network.target

[Service]
ExecStartPre=/sbin/modprobe overlay
ExecStart=/usr/local/bin/containerd
Delegate=yes
KillMode=process
LimitNOFILE=1048576
# Having non-zero Limit*s causes performance problems due to accounting overhead
# in the kernel. We recommend using cgroups to do container-local accounting.
LimitNPROC=infinity
LimitCORE=infinity

[Install]
WantedBy=multi-user.target`

const containerdConfig = `disabled_plugins = ["cri"]

[metrics]
        address = "0.0.0.0:9200"
        grpc_histogram = true

[plugins.cgroups]
        no_prom = false`

var containerdCommand = cli.Command{
	Name:  "containerd",
	Usage: "install containerd on a system",
	Action: func(clix *cli.Context) error {
		dir, err := ioutil.TempDir("", "containerd-install")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		ctx := system.Context()
		cs, err := local.NewStore(dir)
		if err != nil {
			return err
		}
		desc, err := localFetch(ctx, cs)
		if err != nil {
			return err
		}
		platform := platforms.Default()
		manifest, err := images.Manifest(ctx, cs, *desc, platform)
		if err != nil {
			return err
		}
		for _, layer := range manifest.Layers {
			ra, err := cs.ReaderAt(ctx, layer)
			if err != nil {
				return err
			}
			cr := content.NewReader(ra)
			r, err := compression.DecompressStream(cr)
			if err != nil {
				return err
			}
			defer r.Close()
			if _, err := archive.Apply(ctx, "/usr/local", r, archive.WithFilter(func(hdr *tar.Header) (bool, error) {
				d := filepath.Dir(hdr.Name)
				return d == "bin", nil
			})); err != nil {
				return err
			}
		}
		if err := os.MkdirAll("/etc/containerd", 0711); err != nil {
			return err
		}
		f, err := os.Create(filepath.Join("/etc/containerd/config.toml"))
		if err != nil {
			return err
		}
		_, err = f.WriteString(containerdConfig)
		f.Close()
		if err != nil {
			return err
		}
		const name = "containerd.service"
		if err := writeUnit(name, containerdUnit); err != nil {
			return err
		}
		return startNewService(ctx, name)
	},
}

func localFetch(ctx context.Context, cs content.Store) (*v1.Descriptor, error) {
	resolv := docker.NewResolver(docker.ResolverOptions{})
	name, desc, err := resolv.Resolve(ctx, "docker.io/crosbymichael/containerd:latest")
	if err != nil {
		return nil, err
	}
	f, err := resolv.Fetcher(ctx, name)
	if err != nil {
		return nil, err
	}
	r, err := f.Fetch(ctx, desc)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	h := remotes.FetchHandler(cs, f)
	if err := images.Dispatch(ctx, h, desc); err != nil {
		return nil, err
	}
	return &desc, nil
}
