package main

import (
	"context"
	"html/template"
	"os"
	"path/filepath"
	"time"

	"github.com/containerd/containerd"
	"github.com/crosbymichael/boss/config"
	"github.com/crosbymichael/boss/system"
	"github.com/crosbymichael/boss/systemd"
	"github.com/hashicorp/consul/api"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

type step interface {
	name() string
	run(context.Context, *containerd.Client, *cli.Context) error
	remove(context.Context, *containerd.Client, *cli.Context) error
}

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

const metricsUnit = `[Unit]
Description=prometheus node metrics

[Service]
ExecStart=/opt/containerd/bin/nodeexporter
Restart=always

[Install]
WantedBy=multi-user.target`

type nodeMetricsStep struct {
	config *config.Config
}

func (s *nodeMetricsStep) name() string {
	return "node exporter"
}

func (s *nodeMetricsStep) run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	const name = "nodeexporter.service"
	if err := install(ctx, client, s.config.NodeMetrics.Image, clix); err != nil {
		return err
	}
	if err := writeUnit(name, metricsUnit); err != nil {
		return err
	}
	return startNewService(ctx, name)
}

func (s *nodeMetricsStep) remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	if err := client.ImageService().Delete(ctx, s.config.NodeMetrics.Image); err != nil {
		return err
	}
	const name = "nodeexporter.service"
	return disableService(ctx, name)
}

const buildkitUnit = `[Unit]
Description=buildkit
Documentation=moby/buildkit
After=containerd.service

[Service]
ExecStart=/opt/containerd/bin/buildkitd --containerd-worker=true --oci-worker=false
Restart=always

[Install]
WantedBy=multi-user.target`

type buildkitStep struct {
	config *config.Config
}

func (s *buildkitStep) name() string {
	return "buildkit"
}

func (s *buildkitStep) run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	const name = "buildkit.service"
	if err := install(ctx, client, s.config.Buildkit.Image, clix); err != nil {
		return err
	}
	if err := writeUnit(name, buildkitUnit); err != nil {
		return err
	}
	return startNewService(ctx, name)
}

func (s *buildkitStep) remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	if err := client.ImageService().Delete(ctx, s.config.Buildkit.Image); err != nil {
		return err
	}
	const name = "buildkit.service"
	return disableService(ctx, name)
}

type cniStep struct {
	config *config.Config
}

func (s *cniStep) name() string {
	return "cni"
}

func (s *cniStep) run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	return install(ctx, client, s.config.CNI.Image, clix)
}

func (s *cniStep) remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	if err := client.ImageService().Delete(ctx, s.config.CNI.Image); err != nil {
		return err
	}
	return nil
}

const dhcpUnit = `[Unit]
Description=cni dhcp server

[Service]
ExecStartPre=/bin/rm -f /run/cni/dhcp.sock
ExecStart=/opt/containerd/bin/dhcp daemon
Restart=always

[Install]
WantedBy=multi-user.target`

type dhcpStep struct {
}

func (s *dhcpStep) name() string {
	return "dhcp"
}

func (s *dhcpStep) run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	const name = "cni-dhcp.service"
	if err := writeUnit(name, dhcpUnit); err != nil {
		return err
	}
	return startNewService(ctx, name)
}

func (s *dhcpStep) remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	const name = "cni-dhcp.service"
	return disableService(ctx, name)
}

type mkdirRoot struct {
}

func (s *mkdirRoot) name() string {
	return "mkdir /var/lib/boss"
}

func (s *mkdirRoot) run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	return os.MkdirAll(config.Root, 0711)
}

func (s *mkdirRoot) remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	return os.RemoveAll(config.Root)
}

type bossUnit struct {
}

func (s *bossUnit) name() string {
	return "boss unit"
}

func (s *bossUnit) run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	return systemd.Install()
}

func (s *bossUnit) remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	return systemd.Remove()
}

type registerStep struct {
	id     string
	port   int
	tags   []string
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
		ID:      s.id,
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

func install(ctx context.Context, client *containerd.Client, ref string, clix *cli.Context) error {
	image, err := getImage(ctx, client, ref, clix, nil, false)
	if err != nil {
		return err
	}
	return client.Install(ctx, image, containerd.WithInstallReplace, containerd.WithInstallLibs)
}

func writeUnit(name, data string) error {
	f, err := os.Create(filepath.Join("/lib/systemd/system", name))
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(data)
	return err
}

func startNewService(ctx context.Context, name string) error {
	if err := systemd.Command(ctx, "daemon-reload"); err != nil {
		return err
	}
	if err := systemd.Command(ctx, "enable", name); err != nil {
		return err
	}
	if err := systemd.Command(ctx, "start", name); err != nil {
		return err
	}
	t := time.After(10 * time.Second)
	for {
		select {
		case <-t:
			return errors.Errorf("service %s not started", name)
		default:
			if err := systemd.Command(ctx, "status", name); err == nil {
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func disableService(ctx context.Context, name string) error {
	if err := systemd.Command(ctx, "stop", name); err != nil {
		return err
	}
	if err := systemd.Command(ctx, "disable", name); err != nil {
		return err
	}
	return os.Remove(filepath.Join("/lib/systemd/system", name))
}
