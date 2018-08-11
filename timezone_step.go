package main

import (
	"context"
	"os/exec"

	"github.com/containerd/containerd"
	"github.com/crosbymichael/boss/config"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

type timezoneStep struct {
	config *config.Config
}

func (s *timezoneStep) name() string {
	return "timezone"
}

func (s *timezoneStep) run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	tz := s.config.Timezone
	if tz == "" {
		return nil
	}
	out, err := exec.CommandContext(ctx, "timedatectl", "set-timezone", tz).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}

func (s *timezoneStep) remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	return nil
}
