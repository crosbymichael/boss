package config

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/containerd/containerd"
	"github.com/crosbymichael/boss/systemd"
	"github.com/urfave/cli"
)

const sshdConfig = `PermitRootLogin no
PasswordAuthentication no
ChallengeResponseAuthentication no
UsePAM yes
X11Forwarding yes
PrintMotd no
AcceptEnv LANG LC_*
Subsystem       sftp    /usr/lib/openssh/sftp-server`

type SSH struct {
	Admin  string `toml:"admin"`
	Config bool   `toml:"sshd_config"`
	// AuthorizedKeys string `toml:"authorized_keys"`
}

func (m *SSH) Name() string {
	return "ssh"
}

func (m *SSH) Run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	if m.Config {
		if err := ioutil.WriteFile("/etc/ssh/sshd_config", []byte(sshdConfig), 0644); err != nil {
			return err
		}
		if err := systemd.Command(ctx, "restart", "sshd"); err != nil {
			return err
		}
	}
	home := filepath.Join(os.Getenv("HOME"), ".ssh", "authorized_keys")
	if err := os.MkdirAll(filepath.Dir(home), 0775); err != nil {
		return err
	}
	return ioutil.WriteFile(home, []byte(m.Admin), 0664)
}

func (s *SSH) Remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	return nil
}
