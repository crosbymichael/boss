package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/containerd/containerd"
	"github.com/crosbymichael/boss/config"
	"github.com/crosbymichael/boss/system"
	"github.com/hashicorp/consul/api"
	"github.com/urfave/cli"
)

const consulUnit = `[Unit]
Description=consul.io

[Service]
ExecStart=/opt/containerd/bin/consul agent {{.Bootstrap}} -server -data-dir=/var/lib/consul -datacenter {{.Domain}} -node {{.ID}} -ui -bind {{.IP}} -client "127.0.0.1 {{.IP}}" -domain {{.Domain}} -recursor 8.8.8.8 -recursor 8.8.4.4 -dns-port 53
Restart=always

[Install]
WantedBy=multi-user.target`

type consulStep struct {
	config *config.Config
}

func (s *consulStep) name() string {
	return "consul"
}

func (s *consulStep) run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	if err := install(ctx, client, s.config.Consul.Image, clix); err != nil {
		return err
	}
	const name = "consul.service"
	if err := os.MkdirAll("/var/lib/consul", 0711); err != nil {
		return err
	}
	ip, err := system.GetIP(s.config.Iface)
	if err != nil {
		return err
	}
	var tmplCtx = struct {
		Bootstrap string
		Domain    string
		ID        string
		IP        string
	}{
		ID:     s.config.ID,
		Domain: s.config.Domain,
		IP:     ip,
	}
	if s.config.Consul.Bootstrap {
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

func (s *consulStep) remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	consul, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return err
	}
	if err := consul.Agent().Leave(); err != nil {
		return err
	}
	if err := client.ImageService().Delete(ctx, s.config.Consul.Image); err != nil {
		return err
	}
	const name = "consul.service"
	if err := disableService(ctx, name); err != nil {
		return err
	}
	return os.RemoveAll("/var/lib/consul")
}

type joinStep struct {
	ips []string
}

func (s *joinStep) name() string {
	return "join cluster"
}

func (s *joinStep) run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	consul, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return err
	}
	for _, ip := range s.ips {
		if err := consul.Agent().Join(ip, false); err != nil {
			return err
		}
	}
	return nil
}

func (s *joinStep) remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	return nil
}

type registerStep struct {
	id     string
	port   int
	tags   []string
	url    string
	config *config.Config
}

func (s *registerStep) name() string {
	return "register " + s.id
}

func (s *registerStep) run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	ip, err := system.GetIP(s.config.Iface)
	if err != nil {
		return err
	}
	consul, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return err
	}
	reg := &api.AgentServiceRegistration{
		ID:      fmt.Sprintf("%s-%s", s.id, s.config.ID),
		Name:    s.id,
		Tags:    s.tags,
		Port:    s.port,
		Address: ip,
	}
	return consul.Agent().ServiceRegister(reg)
}

func (s *registerStep) remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	consul, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return nil
	}
	consul.Agent().ServiceDeregister(s.id)
	return nil
}
