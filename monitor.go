package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const (
	statusLabel = "io.boss/restart.status"
	// custom boss statuses
	DeleteStatus containerd.ProcessStatus = "delete"
)

type change interface {
	apply(context.Context, *containerd.Client) error
}

type monitor struct {
	client     *containerd.Client
	register   Register
	networking map[NetworkType]Network
	shutdownCh chan struct{}
	mu         sync.Mutex
}

func (m *monitor) shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctx := context.Background()
	ns, err := m.client.NamespaceService().List(ctx)
	if err != nil {
		logrus.WithError(err).Error("list namespaces")
		close(m.shutdownCh)
		return
	}
	for _, name := range ns {
		ctx = namespaces.WithNamespace(ctx, name)
		if err := m.stopContainers(ctx); err != nil {
			logrus.WithError(err).Errorf("attach task in %s", name)
		}
	}
	close(m.shutdownCh)
}

func (m *monitor) stopContainers(ctx context.Context) error {
	containers, err := m.client.Containers(ctx, fmt.Sprintf("labels.%q", statusLabel))
	if err != nil {
		return err
	}
	wg := &sync.WaitGroup{}
	for _, c := range containers {
		task, err := c.Task(ctx, nil)
		if err != nil {
			if errdefs.IsNotFound(err) {
				continue
			}
			logrus.WithError(err).Errorf("load task %s", c.ID())
			continue
		}
		wait, err := task.Wait(ctx)
		if err != nil {
			logrus.WithError(err).Errorf("wait task %s", c.ID())
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := task.Kill(ctx, unix.SIGTERM); err != nil {
				logrus.WithError(err).Errorf("kill task %s", c.ID())
				return
			}
			select {
			case <-wait:
				task.Delete(ctx)
				return
			case <-time.After(10 * time.Second):
				return
			}
		}()
	}
	wg.Wait()
	return nil
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
			if errdefs.IsNotFound(err) {
				continue
			}
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

		m.mu.Lock()
		select {
		case <-m.shutdownCh:
			logrus.Debug("ending reconcile loop for shutdown")
			return
		default:
			if err := m.reconcile(context.Background()); err != nil {
				logrus.WithError(err).Error("reconcile")
			}
		}
		m.mu.Unlock()
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
				container: c,
				m:         m,
			})
		case containerd.Stopped:
			changes = append(changes, &stopChange{
				container: c,
				m:         m,
			})
		case DeleteStatus:
			changes = append(changes, &deleteChange{
				container: c,
				m:         m,
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
