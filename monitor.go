package main

import (
	"context"
	"fmt"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	cni "github.com/containerd/go-cni"
	"github.com/hashicorp/consul/api"
	"github.com/sirupsen/logrus"
)

const (
	statusLabel     = "io.boss/restart.status"
	configExtention = "io.boss/config"
)

type change interface {
	apply(context.Context, *containerd.Client) error
}

type monitor struct {
	client     *containerd.Client
	consul     *api.Client
	networking cni.CNI
}

func (m *monitor) attach() error {
	ctx := context.Background()
	ns, err := m.client.NamespaceService().List(ctx)
	if err != nil {
		return err
	}
	for _, name := range ns {
		ctx = namespaces.WithNamespace(ctx, name)
		if err := m.attachContainers(ctx); err != nil {
			logrus.WithError(err).Errorf("attach task in %s", name)
		}
	}
	return nil
}

func (m *monitor) attachContainers(ctx context.Context) error {
	containers, err := m.client.Containers(ctx, fmt.Sprintf("labels.%q", statusLabel))
	if err != nil {
		return err
	}
	for _, c := range containers {
		task, err := c.Task(ctx, cio.NewAttach(cio.WithStdio))
		if err != nil {
			logrus.WithError(err).Errorf("load task %s", c.ID())
			continue
		}
		logrus.WithFields(logrus.Fields{
			"pid": task.Pid(),
			"id":  task.ID(),
		}).Info("attach task")
	}
	return nil
}

func (m *monitor) run(interval time.Duration) {
	if interval == 0 {
		interval = 10 * time.Second
	}
	for {
		time.Sleep(interval)
		logrus.Debug("reconciling")
		if err := m.reconcile(context.Background()); err != nil {
			logrus.WithError(err).Error("reconcile")
		}
	}
}

func (m *monitor) reconcile(ctx context.Context) error {
	ns, err := m.client.NamespaceService().List(ctx)
	if err != nil {
		return err
	}
	for _, name := range ns {
		ctx = namespaces.WithNamespace(ctx, name)
		changes, err := m.monitor(ctx)
		if err != nil {
			logrus.WithError(err).Error("get changes")
			continue
		}
		for _, c := range changes {
			if err := c.apply(ctx, m.client); err != nil {
				logrus.WithError(err).Error("apply change")
			}
		}
	}
	return nil
}

func (m *monitor) monitor(ctx context.Context) ([]change, error) {
	containers, err := m.client.Containers(ctx, fmt.Sprintf("labels.%q", statusLabel))
	if err != nil {
		return nil, err
	}
	var changes []change
	for _, c := range containers {
		labels, err := c.Labels(ctx)
		if err != nil {
			logrus.WithError(err).Errorf("fetch labels for %s", c.ID())
			continue
		}
		desiredStatus := containerd.ProcessStatus(labels[statusLabel])
		if m.isSameStatus(ctx, desiredStatus, c) {
			continue
		}
		switch desiredStatus {
		case containerd.Running:
			changes = append(changes, &startChange{
				container:  c,
				networking: m.networking,
				consul:     m.consul,
			})
		case containerd.Stopped:
			changes = append(changes, &stopChange{
				container:  c,
				networking: m.networking,
				consul:     m.consul,
			})
		}
	}
	return changes, nil
}

func (m *monitor) isSameStatus(ctx context.Context, desired containerd.ProcessStatus, container containerd.Container) bool {
	task, err := container.Task(ctx, nil)
	if err != nil {
		return desired == containerd.Stopped
	}
	state, err := task.Status(ctx)
	if err != nil {
		return desired == containerd.Stopped
	}
	return desired == state.Status
}
