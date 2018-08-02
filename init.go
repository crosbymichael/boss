package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/namespaces"
	"github.com/urfave/cli"
)

type agentContext struct {
	Interval    string
	Nameservers []string
	Register    string
}

func (a *agentContext) nameservers() string {
	var p []string
	for _, n := range a.Nameservers {
		p = append(p, "--nameservers", n)
	}
	return strings.Join(p, " ")
}

const agentTemplate = `[Unit]
Description=boss agent
Requires=containerd.service
After=containerd.service

[Service]
ExecStart=/usr/local/bin/boss --register {{.Register}} agent --interval {{.Interval}} {{nameservers}}
Restart=always
MemoryLimit=128m

[Install]
WantedBy=multi-user.target`

const buildkitTemplate = `[Unit]
Description=buildkit
Documentation=moby/buildkit
After=containerd.service

[Service]
ExecStart=/opt/containerd/bin/buildkitd --containerd-worker=true --oci-worker=false
Restart=always
MemoryLimit=128m

[Install]
WantedBy=multi-user.target`

const dhcpTemplate = `[Unit]
Description=cni dhcp server

[Service]
ExecPreStart=/usr/bin/rm -f /run/cni/dhcp.sock
ExecStart=/opt/containerd/bin/dhcp daemon
Restart=always
MemoryLimit=128m

[Install]
WantedBy=multi-user.target`

var initCommand = cli.Command{
	Name:  "init",
	Usage: "init boss on a system",
	Subcommands: []cli.Command{
		initAgentCommand,
		initBuildkitCommand,
		initCNICommand,
	},
}

var initAgentCommand = cli.Command{
	Name:  "agent",
	Usage: "init boss agent on a system",
	Flags: []cli.Flag{
		cli.DurationFlag{
			Name:  "interval,i",
			Usage: "set the interval to reconcile state",
			Value: 10 * time.Second,
		},
		cli.StringSliceFlag{
			Name:  "nameservers,n",
			Usage: "set the boss nameservers",
			Value: &cli.StringSlice{
				"8.8.8.8",
				"8.8.4.4",
			},
		},
	},
	Action: func(clix *cli.Context) error {
		ac := &agentContext{
			Interval:    clix.Duration("interval").String(),
			Nameservers: clix.StringSlice("nameservers"),
			Register:    clix.GlobalString("register"),
		}
		t, err := template.New("agent").Funcs(template.FuncMap{
			"nameservers": ac.nameservers,
		}).Parse(agentTemplate)
		if err != nil {
			return err
		}
		f, err := os.Create(filepath.Join("/lib/systemd/system", "boss-agent.service"))
		if err != nil {
			return err
		}
		err = t.Execute(f, ac)
		f.Close()
		if err != nil {
			return err
		}
		if err := systemd("enable", "boss-agent"); err != nil {
			return err
		}
		return systemd("start", "boss-agent")
	},
}

var initBuildkitCommand = cli.Command{
	Name:  "buildkit",
	Usage: "init buildkit on a system",
	Action: func(clix *cli.Context) error {
		ctx := namespaces.WithNamespace(context.Background(), clix.GlobalString("namespace"))
		client, err := containerd.New(
			defaults.DefaultAddress,
			containerd.WithDefaultRuntime("io.containerd.runc.v1"),
		)
		if err != nil {
			return err
		}
		defer client.Close()
		image, err := getImage(ctx, client, "docker.io/crosbymichael/buildkit:latest", clix)
		if err != nil {
			return err
		}
		if err := client.Install(ctx, image); err != nil {
			return err
		}
		f, err := os.Create(filepath.Join("/lib/systemd/system", "buildkit.service"))
		if err != nil {
			return err
		}
		_, err = f.WriteString(buildkitTemplate)
		f.Close()
		if err != nil {
			return err
		}
		if err := systemd("enable", "buildkit"); err != nil {
			return err
		}
		return systemd("start", "buildkit")
	},
}

var initCNICommand = cli.Command{
	Name:  "cni",
	Usage: "init cni on a system",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "dhcp",
			Usage: "start the dhcp server",
		},
		cli.StringSliceFlag{
			Name:  "networks",
			Usage: "add cni network configurations",
			Value: &cli.StringSlice{},
		},
	},
	Action: func(clix *cli.Context) error {
		ctx := namespaces.WithNamespace(context.Background(), clix.GlobalString("namespace"))
		client, err := containerd.New(
			defaults.DefaultAddress,
			containerd.WithDefaultRuntime("io.containerd.runc.v1"),
		)
		if err != nil {
			return err
		}
		defer client.Close()
		image, err := getImage(ctx, client, "docker.io/crosbymichael/cni:latest", clix)
		if err != nil {
			return err
		}
		if err := client.Install(ctx, image); err != nil {
			return err
		}
		if err := installNetworks(clix.StringSlice("networks")); err != nil {
			return err
		}
		if !clix.Bool("dhcp") {
			return nil
		}
		f, err := os.Create(filepath.Join("/lib/systemd/system", "cni-dhcp.service"))
		if err != nil {
			return err
		}
		_, err = f.WriteString(dhcpTemplate)
		f.Close()
		if err != nil {
			return err
		}
		if err := systemd("enable", "cni-dhcp"); err != nil {
			return err
		}
		return systemd("start", "cni-dhcp")
	},
}

func installNetworks(networks []string) error {
	path := "/etc/cni/net.d"
	if err := os.MkdirAll(path, 0711); err != nil {
		return err
	}
	for _, name := range networks {
		f, err := os.Create(filepath.Join(path, name))
		if err != nil {
			return err
		}
		data, err := os.Open(name)
		if err != nil {
			f.Close()
			return err
		}
		if _, err := io.Copy(f, data); err != nil {
			f.Close()
			data.Close()
		}
		f.Close()
		data.Close()
	}
	return nil
}

func systemd(action, name string) error {
	cmd := exec.Command("systemctl", action, name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s", err, out)
	}
	return nil
}
