package main

import (
	"context"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/containerd/containerd"
	"github.com/hashicorp/consul/api"
	"github.com/urfave/cli"
)

const consulUnit = `[Unit]
Description=consul.io

[Service]
ExecStart=/opt/containerd/bin/consul agent {{.Bootstrap}} -server -data-dir=/var/lib/consul -datacenter {{.Domain}} -node {{.ID}} -ui -client "127.0.0.1 {{.IP}}" -domain {{.Domain}} -recursor 8.8.8.8 -recursor 8.8.4.4 -dns-port 53
Restart=always

[Install]
WantedBy=multi-user.target`

type consulStep struct {
}

func (s *consulStep) run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	if err := install(ctx, client, cfg.Consul.Image, clix); err != nil {
		return err
	}
	const name = "consul.service"
	if err := os.MkdirAll("/var/lib/consul", 0711); err != nil {
		return err
	}
	var tmplCtx = struct {
		Bootstrap string
		Domain    string
		ID        string
		IP        string
	}{
		ID:     cfg.ID,
		Domain: cfg.Domain,
		IP:     cfg.IP(),
	}
	if cfg.Consul.Bootstrap {
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
	return startNewService(name)
}

type joinStep struct {
	ips []string
}

func (s *joinStep) run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	cmd := exec.CommandContext(ctx, "consul", append([]string{"join"}, s.ips...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s", err, out)
	}
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
}

func (s *nodeMetricsStep) run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	const name = "nodeexporter.service"
	if err := install(ctx, client, cfg.NodeMetrics.Image, clix); err != nil {
		return err
	}
	if err := writeUnit(name, metricsUnit); err != nil {
		return err
	}
	return startNewService(name)
}

const buildkitUnit = `[Unit]
Description=buildkit
Documentation=moby/buildkit
After=containerd.service

[Service]
ExecStart=/opt/containerd/bin/buildkitd --containerd-worker=true --oci-worker=false --addr 0.0.0.0:9000
Restart=always

[Install]
WantedBy=multi-user.target`

type buildkitStep struct {
}

func (s *buildkitStep) run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	const name = "buildkit.service"
	if err := install(ctx, client, cfg.Buildkit.Image, clix); err != nil {
		return err
	}
	if err := writeUnit(name, buildkitUnit); err != nil {
		return err
	}
	return startNewService(name)
}

type cniStep struct {
}

func (s *cniStep) run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	return install(ctx, client, cfg.CNI.Image, clix)
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

func (s *dhcpStep) run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	const name = "cni-dhcp.service"
	if err := writeUnit(name, dhcpUnit); err != nil {
		return err
	}
	return startNewService(name)
}

const agentUnit = `[Unit]
Description=boss agent
Requires=containerd.service
After=containerd.service

[Service]
ExecStart=/usr/local/bin/boss agent
Restart=always

[Install]
WantedBy=multi-user.target`

type agentStep struct {
}

func (s *agentStep) run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	const name = "boss-agent.service"
	if err := writeUnit(name, agentUnit); err != nil {
		return err
	}
	return startNewService(name)
}

type registerStep struct {
	id   string
	name string
	port int
	tags []string
}

func (s *registerStep) run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	consul, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return err
	}
	reg := &api.AgentServiceRegistration{
		ID:      s.id,
		Name:    s.name,
		Tags:    s.tags,
		Port:    s.port,
		Address: cfg.IP(),
	}
	return consul.Agent().ServiceRegister(reg)
}

func install(ctx context.Context, client *containerd.Client, ref string, clix *cli.Context) error {
	image, err := getImage(ctx, client, ref, clix)
	if err != nil {
		return err
	}
	return client.Install(ctx, image)
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

func startNewService(name string) error {
	if err := systemd("enable", name); err != nil {
		return err
	}
	return systemd("start", name)
}

func systemd(action, name string) error {
	cmd := exec.Command("systemctl", action, name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s", err, out)
	}
	return nil
}
