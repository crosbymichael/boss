package main

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	cni "github.com/containerd/go-cni"
	"github.com/containerd/typeurl"
	"github.com/hashicorp/consul/api"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

type stopChange struct {
	container  containerd.Container
	networking cni.CNI
	consul     *api.Client
}

func (s *stopChange) apply(ctx context.Context, client *containerd.Client) error {
	if err := killTask(ctx, s.container); err != nil {
		return err
	}
	return nil
}

type startChange struct {
	container  containerd.Container
	networking cni.CNI
	consul     *api.Client
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

	if !config.HostNetwork {
		result, err := s.networking.Setup(s.container.ID(), fmt.Sprintf("/proc/%d/ns/net", task.Pid))
		if err != nil {
			if _, derr := task.Delete(ctx, containerd.WithProcessKill); derr != nil {
				logrus.WithError(derr).Error("delete task on failed network setup")
			}
			return err
		}
		var ip net.IP
		for _, ipc := range result.Interfaces["eth0"].IPConfigs {
			if f := ipc.IP.To4(); f != nil {
				ip = f
				break
			}
		}
		logrus.WithField("id", config.ID).WithField("ip", ip).Info("setup network interface")
		for name, srv := range config.Services {
			reg := &api.AgentServiceRegistration{
				ID:      config.ID,
				Name:    name,
				Tags:    srv.Labels,
				Port:    srv.Port,
				Address: ip.String(),
			}
			if err := s.consul.Agent().ServiceRegister(reg); err != nil {
				logrus.WithError(err).Error("register service")
			}
		}
		if err := s.consul.Agent().EnableServiceMaintenance(config.ID, "starting"); err != nil {
			logrus.WithError(err).Error("enable service maintenance")
		}
	}
	if err := task.Start(ctx); err != nil {
		return err
	}
	if err := s.consul.Agent().DisableServiceMaintenance(config.ID); err != nil {
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
