package monitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/errdefs"
	"github.com/crosbymichael/boss/system"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const (
	StatusLabel = "io.boss/restart.status"
	// custom boss status
	DeleteStatus containerd.ProcessStatus = "delete"
)

// New returns a new monitor for containers
func New(cfg *system.Config) *Monitor {
	return &Monitor{
		config:     cfg,
		shutdownCh: make(chan struct{}, 1),
	}
}

type Monitor struct {
	mu sync.Mutex

	config     *system.Config
	shutdownCh chan struct{}
}

func (m *Monitor) Stop() {
	close(m.shutdownCh)
}

func (m *Monitor) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.stopContainers(m.config.Context()); err != nil {
		logrus.WithError(err).Error("stop tasks")
	}
	close(m.shutdownCh)
}

func (m *Monitor) stopContainers(ctx context.Context) error {
	containers, err := m.config.Client().Containers(ctx, fmt.Sprintf("labels.%q", StatusLabel))
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

func (m *Monitor) Attach() error {
	return m.attachContainers(m.config.Context())
}

func (m *Monitor) attachContainers(ctx context.Context) error {
	containers, err := m.config.Client().Containers(ctx, fmt.Sprintf("labels.%q", StatusLabel))
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

func (m *Monitor) Run() error {
	for {
		time.Sleep(time.Duration(m.config.Agent.Interval) * time.Second)

		m.mu.Lock()
		select {
		case <-m.shutdownCh:
			logrus.Debug("ending reconcile loop for shutdown")
			return nil
		default:
			if err := m.reconcile(m.config.Context()); err != nil {
				logrus.WithError(err).Error("reconcile")
			}
		}
		m.mu.Unlock()
	}
}

func (m *Monitor) reconcile(ctx context.Context) error {
	changes, err := m.monitor(ctx)
	if err != nil {
		return err
	}
	for _, c := range changes {
		if err := c.apply(ctx, m.config.Client()); err != nil {
			logrus.WithError(err).Error("apply change")
		}
	}
	return nil
}

func (m *Monitor) monitor(ctx context.Context) ([]change, error) {
	containers, err := m.config.Client().Containers(ctx, fmt.Sprintf("labels.%q", StatusLabel))
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
		desiredStatus := containerd.ProcessStatus(labels[StatusLabel])
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

func (m *Monitor) isSameStatus(ctx context.Context, desired containerd.ProcessStatus, container containerd.Container) bool {
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
