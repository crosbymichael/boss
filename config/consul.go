package config

import (
	"context"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/containerd/containerd"
	"github.com/crosbymichael/boss/util"
	"github.com/hashicorp/consul/api"
	"github.com/urfave/cli"
)

const consulUnit = `[Unit]
Description=consul.io
After=network.target

[Service]
ExecStart=/opt/containerd/bin/consul agent {{.Bootstrap}} {{.Server}} -data-dir=/var/lib/consul -datacenter {{.Domain}} -node {{.ID}} -ui -bind {{.IP}} -client "127.0.0.1 {{.IP}}" -domain {{.Domain}} -recursor 8.8.8.8 -recursor 8.8.4.4 -dns-port 53
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target`

type Consul struct {
	Image    string   `toml:"image"`
	Join     []string `toml:"join"`
	NoServer bool     `toml:"no_server"`
	c        *Config
}

func (s *Consul) SubSteps() (o []Step) {
	if len(s.Join) > 0 {
		o = append(o, &Join{
			IPs: s.Join,
		},
			&RegisterService{
				Config: s.c,
				ID:     "containerd",
				Tags: []string{
					"metrics",
				},
				Port: 9200,
			},
		)
	}

	return o
}

func (s *Consul) Name() string {
	return "consul"
}

func (s *Consul) Run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	if err := install(ctx, client, s.Image, clix); err != nil {
		return err
	}
	const name = "consul.service"
	if err := os.MkdirAll("/var/lib/consul", 0711); err != nil {
		return err
	}
	ip, err := util.GetIP(s.c.Iface)
	if err != nil {
		return err
	}
	var tmplCtx = struct {
		Bootstrap string
		Domain    string
		ID        string
		IP        string
		Server    string
	}{
		ID:     s.c.ID,
		Domain: s.c.Domain,
		IP:     ip,
	}
	if len(s.Join) == 0 {
		tmplCtx.Bootstrap = "-bootstrap"
	}
	if !s.NoServer {
		tmplCtx.Server = "-server"
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
	if err := startNewService(ctx, name); err != nil {
		return err
	}
	time.Sleep(5 * time.Second)
	return nil
}

func (s *Consul) Remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	consul, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return err
	}
	if err := consul.Agent().Leave(); err != nil {
		return err
	}
	if err := client.ImageService().Delete(ctx, s.Image); err != nil {
		return err
	}
	const name = "consul.service"
	if err := disableService(ctx, name); err != nil {
		return err
	}
	return os.RemoveAll("/var/lib/consul")
}
