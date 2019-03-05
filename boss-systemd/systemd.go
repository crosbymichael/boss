package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/contrib/apparmor"
	"github.com/containerd/containerd/errdefs"
	v1 "github.com/crosbymichael/boss/api/v1"
	"github.com/crosbymichael/boss/config"
	"github.com/crosbymichael/boss/opts"
	"github.com/crosbymichael/boss/system"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	errIDRequired     = errors.New("container id is required")
	errUnableToSignal = errors.New("unable to signal task")
)

var systemdExecStartPreCommand = cli.Command{
	Name:  "exec-start-pre",
	Usage: "exec-start-pre proxy for containers",
	Action: func(clix *cli.Context) error {
		id := clix.Args().First()
		c, err := config.Load()
		if err != nil {
			return err
		}
		if err := setupResolvConf(id, c); err != nil {
			return err
		}
		if err := setupApparmor(); err != nil {
			return err
		}
		return cleanupPreviousTask(id)
	},
}

var systemdExecStopPostCommand = cli.Command{
	Name:  "exec-stop-post",
	Usage: "exec-stop-post proxy for containers",
	Action: func(clix *cli.Context) error {
		id := clix.Args().First()
		err := cleanupPreviousTask(id)
		ctx := system.Context()
		c, err := config.Load()
		if err != nil {
			return err
		}
		client, err := system.NewClient()
		if err != nil {
			return err
		}
		defer client.Close()
		container, err := client.LoadContainer(ctx, id)
		if err != nil {
			return err
		}
		config, err := opts.GetConfig(ctx, container)
		if err != nil {
			return err
		}
		register, err := c.GetRegister()
		if err != nil {
			return err
		}
		for name := range config.Services {
			register.EnableMaintainance(id, name, "task exited")
		}
		return err
	},
}

var systemdExecStartCommand = cli.Command{
	Name:  "exec-start",
	Usage: "exec-start proxy for containers",
	Action: func(clix *cli.Context) error {
		id := clix.Args().First()
		var (
			signals = make(chan os.Signal, 64)
			ctx     = system.Context()
		)
		signal.Notify(signals)
		client, err := system.NewClient()
		if err != nil {
			return err
		}
		defer client.Close()
		container, err := client.LoadContainer(ctx, id)
		if err != nil {
			return err
		}
		desc, err := opts.GetRestoreDesc(ctx, container)
		if err != nil {
			return err
		}
		cfg, err := opts.GetConfig(ctx, container)
		if err != nil {
			return err
		}
		c, err := config.Load()
		if err != nil {
			return err
		}
		register, err := c.GetRegister()
		if err != nil {
			return err
		}
		store, err := c.Store()
		if err != nil {
			return err
		}
		templateCh, err := store.Watch(ctx, container, cfg)
		if err != nil {
			return err
		}
		ip, err := setupNetworking(ctx, container, cfg)
		if err != nil {
			return err
		}
		if err := container.Update(ctx, opts.WithIP(ip), opts.WithoutRestore); err != nil {
			return err
		}
		task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStdio), opts.WithTaskRestore(desc))
		if err != nil {
			return err
		}
		status, err := monitorTask(ctx, client, task, cfg, register, signals, templateCh)
		if err != nil {
			return err
		}
		os.Exit(status)
		return nil
	},
}

func monitorTask(ctx context.Context, client *containerd.Client, task containerd.Task, config *v1.Container, register v1.Register, signals chan os.Signal, templateCh <-chan error) (int, error) {
	defer task.Delete(ctx, containerd.WithProcessKill)
	started := make(chan error, 1)
	wait, err := task.Wait(ctx)
	if err != nil {
		return -1, err
	}
	go func() {
		started <- task.Start(ctx)
	}()
	for {
		select {
		case err := <-templateCh:
			logrus.WithError(err).Error("render template")
		case err := <-started:
			if err != nil {
				return -1, err
			}
			for name := range config.Services {
				if err := register.DisableMaintainance(task.ID(), name); err != nil {
					logrus.WithError(err).Error("disable service maintenance")
				}
			}
		case s := <-signals:
			if err := trySendSignal(ctx, client, task, s); err != nil {
				logrus.WithError(err).Error("signal task")
			}
		case exit := <-wait:
			if exit.Error() != nil {
				if !isUnavailable(err) {
					return -1, err
				}
				if err := reconnect(client); err != nil {
					return -1, err
				}
				if wait, err = task.Wait(ctx); err != nil {
					return -1, err
				}
				continue
			}
			return int(exit.ExitCode()), nil
		}
	}
}

func trySendSignal(ctx context.Context, client *containerd.Client, task containerd.Task, s os.Signal) error {
	for i := 0; i < 5; i++ {
		err := task.Kill(ctx, s.(syscall.Signal))
		if err == nil {
			return nil
		}
		if !isUnavailable(err) {
			return err
		}
		if err := reconnect(client); err != nil {
			return err
		}
	}
	return errUnableToSignal
}

func reconnect(client *containerd.Client) (err error) {
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		if err = client.Reconnect(); err == nil {
			return nil
		}
	}
	return err
}

func isUnavailable(err error) bool {
	return errdefs.IsUnavailable(errdefs.FromGRPC(err))
}

func setupNetworking(ctx context.Context, container containerd.Container, c *v1.Container) (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}
	network, err := cfg.GetNetwork(c.Network)
	if err != nil {
		return "", err
	}
	register, err := cfg.GetRegister()
	if err != nil {
		return "", err
	}
	ip, err := network.Create(ctx, container)
	if err != nil {
		return "", err
	}
	if ip != "" {
		logrus.WithField("id", container.ID()).WithField("ip", ip).Info("setup network interface")
		for name, srv := range c.Services {
			logrus.WithField("id", container.ID()).WithField("ip", ip).Infof("registering %s", name)
			if err := register.Register(container.ID(), name, ip, srv); err != nil {
				return ip, err
			}
		}
	}
	return ip, nil
}

func systemdPreSetup(clix *cli.Context) error {
	id := clix.Args().First()
	if id == "" {
		return errIDRequired
	}
	return nil
}

func setupResolvConf(id string, c *config.Config) error {
	servers, err := c.GetNameservers()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(v1.Root, id), 0711); err != nil {
		return err
	}
	f, err := ioutil.TempFile("", "boss-resolvconf")
	if err != nil {
		return err
	}
	if err := f.Chmod(0666); err != nil {
		return err
	}
	for _, ns := range servers {
		if _, err := f.WriteString(fmt.Sprintf("nameserver %s\n", ns)); err != nil {
			f.Close()
			return err
		}
	}
	f.Close()
	return os.Rename(f.Name(), filepath.Join(v1.Root, id, "resolv.conf"))
}

func setupApparmor() error {
	return apparmor.WithDefaultProfile("boss")(nil, nil, nil, &specs.Spec{
		Process: &specs.Process{},
	})
}

func cleanupPreviousTask(id string) error {
	ctx := system.Context()
	client, err := system.NewClient()
	if err != nil {
		return err
	}
	defer client.Close()
	container, err := client.LoadContainer(ctx, id)
	if err != nil {
		return err
	}
	task, err := container.Task(ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return err
	}
	_, err = task.Delete(ctx, containerd.WithProcessKill)
	return err
}
