package main

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/containerd/containerd"
	"github.com/crosbymichael/boss/config"
	"github.com/crosbymichael/boss/systemd"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

type step interface {
	name() string
	run(context.Context, *containerd.Client, *cli.Context) error
	remove(context.Context, *containerd.Client, *cli.Context) error
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
