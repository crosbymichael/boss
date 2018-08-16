package config

import (
	"context"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/containerd/containerd"
	"github.com/crosbymichael/boss/system"
	"github.com/crosbymichael/boss/systemd"
	"github.com/pkg/errors"
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

const (
	containerdConfigPath = "/etc/containerd/config.toml"
	containerdPayload    = "https://github.com/crosbymichael/containerd/releases/download/boss/containerd.tar.gz"
)

type Containerd struct {
}

func (s *Containerd) Name() string {
	return "containerd"
}

func (s *Containerd) Run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	const name = "containerd.service"
	if err := systemd.Command(ctx, "status", name); err == nil {
		return nil
	}
	if err := download(containerdPayload); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(containerdConfigPath), 0711); err != nil {
		return err
	}
	if err := ioutil.WriteFile(containerdConfigPath, []byte(containerdConfig), 0666); err != nil {
		return err
	}
	if err := writeUnit(name, containerdUnit); err != nil {
		return err
	}
	if err := startNewService(ctx, name); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)
	client, err := system.NewClient()
	if err != nil {
		return errors.Wrap(err, "unable to connect to containerd, it's the one thing you have to do...for now")
	}
	defer client.Close()
	// install runc
	if err := install(ctx, client, "docker.io/crosbymichael/runc:latest", clix); err != nil {
		return err
	}
	return nil
}

func (s *Containerd) Remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	return nil
}

func download(url string) error {
	r, err := http.Get(containerdPayload)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	cmd := exec.Command("tar", "-C", "/usr/local", "-zxf", "-")
	cmd.Stdin = r.Body
	return cmd.Run()
}
