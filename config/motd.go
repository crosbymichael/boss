package config

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/containerd/containerd"
	"github.com/urfave/cli"
)

const (
	headerPath = "/etc/boss/header"
	header00   = "/etc/update-motd.d"
)

const headerScript = `#!/bin/sh
[ -r /etc/lsb-release ] && . /etc/lsb-release

if [ -z "$DISTRIB_DESCRIPTION" ] && [ -x /usr/bin/lsb_release ]; then
        # Fall back to using the very slow lsb_release utility
        DISTRIB_DESCRIPTION=$(lsb_release -s -d)
fi

cat /etc/boss/header
printf "Hostname: $(hostname)\n"`

var remove = []string{
	"50-motd-news",
	"10-help-text",
	"80-livepatch",
}

type MOTD struct {
	Banner string `toml:"banner"`
}

func (m *MOTD) Name() string {
	return "motd"
}

func (m *MOTD) Run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	if err := ioutil.WriteFile(headerPath, []byte(m.Banner), 0666); err != nil {
		return err
	}
	if err := ioutil.WriteFile(filepath.Join(header00, "00-header"), []byte(headerScript), 0755); err != nil {
		return err
	}
	for _, f := range remove {
		os.Remove(filepath.Join(header00, f))
	}
	return nil
}

func (s *MOTD) Remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	return nil
}
