package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/containerd/containerd"
	"github.com/crosbymichael/boss/image"
	"github.com/crosbymichael/boss/systemd"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

type Step interface {
	Name() string
	Run(context.Context, *containerd.Client, *cli.Context) error
	Remove(context.Context, *containerd.Client, *cli.Context) error
}

func RegisterName(id string) string {
	return fmt.Sprintf("register %s", id)
}

func install(ctx context.Context, client *containerd.Client, ref string, clix *cli.Context) error {
	image, err := image.Get(ctx, client, ref, clix, nil, false)
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
