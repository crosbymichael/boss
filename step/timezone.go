package step

import (
	"context"
	"os/exec"

	"github.com/containerd/containerd"
	"github.com/crosbymichael/boss/config"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

type Timezone struct {
	Config *config.Config
}

func (s *Timezone) Name() string {
	return "timezone"
}

func (s *Timezone) Run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	tz := s.Config.Timezone
	if tz == "" {
		return nil
	}
	out, err := exec.CommandContext(ctx, "timedatectl", "set-timezone", tz).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}

func (s *Timezone) Remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	return nil
}
