package main

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/typeurl"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

type stopChange struct {
	container containerd.Container
	m         *monitor
}

func (s *stopChange) apply(ctx context.Context, client *containerd.Client) error {
	if err := s.m.register.EnableMaintainance(s.container.ID(), "manual stop"); err != nil {
		return err
	}
	if err := killTask(ctx, s.container); err != nil {
		return err
	}
	return nil
}

type startChange struct {
	container containerd.Container
	m         *monitor
}

func (s *startChange) apply(ctx context.Context, client *containerd.Client) error {
	killTask(ctx, s.container)
	config, err := getConfig(ctx, s.container)
	if err != nil {
		return err
	}
	task, err := s.container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
	if err != nil {
		return err
	}
	ip, err := s.m.networking[config.Network].Create(task)
	if err != nil {
		if _, derr := task.Delete(ctx, containerd.WithProcessKill); derr != nil {
			logrus.WithError(derr).Error("delete task on failed network setup")
		}
		return err
	}
	if ip != "" {
		logrus.WithField("id", config.ID).WithField("ip", ip).Info("setup network interface")
		for name, srv := range config.Services {
			if err := s.m.register.Register(config.ID, name, ip, srv); err != nil {
				logrus.WithError(err).Error("register service")
			}
		}
	}
	if err := task.Start(ctx); err != nil {
		return err
	}
	if err := s.m.register.DisableMaintainance(config.ID); err != nil {
		logrus.WithError(err).Error("disable service maintenance")
	}
	return nil
}

func killTask(ctx context.Context, container containerd.Container) error {
	signal := unix.SIGTERM
	task, err := container.Task(ctx, nil)
	if err == nil {
		wait, err := task.Wait(ctx)
		if err != nil {
			if _, derr := task.Delete(ctx); derr == nil {
				return nil
			}
			return err
		}
	kill:
		if err := task.Kill(ctx, signal, containerd.WithKillAll); err != nil {
			if _, derr := task.Delete(ctx); derr == nil {
				return nil
			}
			return err
		}
		select {
		case <-wait:
			if _, err := task.Delete(ctx); err != nil {
				return err
			}
		case <-time.After(10 * time.Second):
			signal = unix.SIGKILL
			goto kill
		}
	}
	return nil
}

func getConfig(ctx context.Context, container containerd.Container) (*Config, error) {
	info, err := container.Info(ctx)
	if err != nil {
		return nil, err
	}
	d := info.Extensions[configExtention]
	v, err := typeurl.UnmarshalAny(&d)
	if err != nil {
		return nil, err
	}
	return v.(*Config), nil
}

type deleteChange struct {
	container containerd.Container
	m         *monitor
}

func (s *deleteChange) apply(ctx context.Context, client *containerd.Client) error {
	path := filepath.Join(rootDir, s.container.ID())
	if err := os.RemoveAll(path); err != nil {
		logrus.WithError(err).Errorf("delete root dir %s", path)
	}
	config, err := getConfig(ctx, s.container)
	if err != nil {
		return err
	}
	s.m.register.Deregister(s.container.ID())
	s.m.networking[config.Network].Remove(s.container)
	return s.container.Delete(ctx, containerd.WithSnapshotCleanup)
}
