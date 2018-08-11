package main

import (
	"context"
	"io/ioutil"
	"os"
	"text/template"

	"github.com/containerd/containerd"
	"github.com/crosbymichael/boss/systemd"
	"github.com/urfave/cli"
)

const hostsFile = `127.0.0.1       localhost.localdomain   localhost {{.ID}}
::1             localhost6.localdomain6 localhost6 aegis-03

# The following lines are desirable for IPv6 capable hosts
::1     localhost ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff02::1 ip6-allnodes
ff02::2 ip6-allrouters
ff02::3 ip6-allhosts`

const resolvConf = `nameserver 127.0.0.1`

type resolvedStep struct {
	ID string
}

func (s *resolvedStep) name() string {
	return "dns"
}

func (s *resolvedStep) run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	if err := systemd.Command(ctx, "disable", "systemd-resolved"); err != nil {
		return err
	}
	if err := systemd.Command(ctx, "stop", "systemd-resolved"); err != nil {
		return err
	}
	t, err := ioutil.TempFile("", "boss-resolvconf")
	if err != nil {
		return err
	}
	_, err = t.WriteString(resolvConf)
	t.Close()
	if err != nil {
		return err
	}
	if err := os.Rename(t.Name(), "/etc/resolv.conf"); err != nil {
		return err
	}
	tmpl, err := template.New("hosts").Parse(hostsFile)
	if err != nil {
		return err
	}
	if t, err = ioutil.TempFile("", "boss-hosts"); err != nil {
		return err
	}
	err = tmpl.Execute(t, s)
	t.Close()
	if err != nil {
		return err
	}
	return os.Rename(t.Name(), "/etc/hosts")
}

func (s *resolvedStep) remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	if err := systemd.Command(ctx, "enable", "systemd-resolved"); err != nil {
		return err
	}
	return systemd.Command(ctx, "start", "systemd-resolved")
}
