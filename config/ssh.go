package config

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/containerd/containerd"
	"github.com/urfave/cli"
)

type SSH struct {
	Admin string `toml:"admin"`
	// AuthorizedKeys string `toml:"authorized_keys"`
}

func (m *SSH) Name() string {
	return "ssh"
}

func (m *SSH) Run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	home := filepath.Join(os.Getenv("HOME"), ".ssh", "authorized_keys")
	if err := os.MkdirAll(filepath.Dir(home), 0775); err != nil {
		return err
	}
	return ioutil.WriteFile(home, []byte(m.Admin), 0664)
}

func (s *SSH) Remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	return nil
}
