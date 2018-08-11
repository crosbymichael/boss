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

const resolvedConfTemplate = `#  This file is part of systemd.
#
#  systemd is free software; you can redistribute it and/or modify it
#  under the terms of the GNU Lesser General Public License as published by
#  the Free Software Foundation; either version 2.1 of the License, or
#  (at your option) any later version.
#
# Entries in this file show the compile time defaults.
# You can change settings by editing this file.
# Defaults can be restored by simply deleting this file.
#
# See resolved.conf(5) for details

[Resolve]
DNS=127.0.0.1
#FallbackDNS=
Domains=~{{.Domain}}
#LLMNR=no
#MulticastDNS=no
#DNSSEC=no
#Cache=yes
#DNSStubListener=yes`

const resolvedConf = `#  This file is part of systemd.
#
#  systemd is free software; you can redistribute it and/or modify it
#  under the terms of the GNU Lesser General Public License as published by
#  the Free Software Foundation; either version 2.1 of the License, or
#  (at your option) any later version.
#
# Entries in this file show the compile time defaults.
# You can change settings by editing this file.
# Defaults can be restored by simply deleting this file.
#
# See resolved.conf(5) for details

[Resolve]
#DNS=
#FallbackDNS=
#Domains=
#LLMNR=no
#MulticastDNS=no
#DNSSEC=no
#Cache=yes
#DNSStubListener=yes`

const resolvedConfigPath = "/etc/systemd/resolved.conf"

type resolvedStep struct {
	Domain string
}

func (s *resolvedStep) name() string {
	return "resolved"
}

func (s *resolvedStep) run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	t, err := ioutil.TempFile("", "boss-resolved")
	if err != nil {
		return err
	}
	tmpl, err := template.New("resolved").Parse(resolvedConfTemplate)
	if err != nil {
		return err
	}
	err = tmpl.Execute(t, s)
	t.Close()
	if err != nil {
		return err
	}
	if err := os.Rename(t.Name(), resolvedConfigPath); err != nil {
		return err
	}
	if err := systemd.Command(ctx, "daemon-reload"); err != nil {
		return err
	}
	return systemd.Command(ctx, "restart", "systemd-resolved")
}

func (s *resolvedStep) remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	if err := os.Remove(resolvedConfigPath); err != nil {
		return err
	}
	if err := systemd.Command(ctx, "daemon-reload"); err != nil {
		return err
	}
	return systemd.Command(ctx, "restart", "systemd-resolved")
}
