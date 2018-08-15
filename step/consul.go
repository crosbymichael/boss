package step

import (
	"context"
	"os"
	"path/filepath"
	"text/template"

	"github.com/containerd/containerd"
	"github.com/crosbymichael/boss/config"
	"github.com/crosbymichael/boss/util"
	"github.com/hashicorp/consul/api"
	"github.com/urfave/cli"
)

const consulUnit = `[Unit]
Description=consul.io
After=network.target

[Service]
ExecStart=/opt/containerd/bin/consul agent {{.Bootstrap}} -server -data-dir=/var/lib/consul -datacenter {{.Domain}} -node {{.ID}} -ui -bind {{.IP}} -client "127.0.0.1 {{.IP}}" -domain {{.Domain}} -recursor 8.8.8.8 -recursor 8.8.4.4 -dns-port 53
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target`

type Consul struct {
	Config    *config.Config
	Bootstrap bool
}

func (s *Consul) Name() string {
	return "consul"
}

func (s *Consul) Run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	if err := install(ctx, client, s.Config.Consul.Image, clix); err != nil {
		return err
	}
	const name = "consul.service"
	if err := os.MkdirAll("/var/lib/consul", 0711); err != nil {
		return err
	}
	ip, err := util.GetIP(s.Config.Iface)
	if err != nil {
		return err
	}
	var tmplCtx = struct {
		Bootstrap string
		Domain    string
		ID        string
		IP        string
	}{
		ID:     s.Config.ID,
		Domain: s.Config.Domain,
		IP:     ip,
	}
	if s.Bootstrap {
		tmplCtx.Bootstrap = "-bootstrap"
	}
	t, err := template.New("consul").Parse(consulUnit)
	if err != nil {
		return err
	}
	f, err := os.Create(filepath.Join("/lib/systemd/system", name))
	if err != nil {
		return err
	}
	err = t.Execute(f, tmplCtx)
	f.Close()
	return startNewService(ctx, name)
}

func (s *Consul) Remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	consul, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return err
	}
	if err := consul.Agent().Leave(); err != nil {
		return err
	}
	if err := client.ImageService().Delete(ctx, s.Config.Consul.Image); err != nil {
		return err
	}
	const name = "consul.service"
	if err := disableService(ctx, name); err != nil {
		return err
	}
	return os.RemoveAll("/var/lib/consul")
}
