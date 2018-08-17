package agent

import (
	"context"
	"os"
	"path/filepath"

	"github.com/containerd/cgroups"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/typeurl"
	"github.com/crosbymichael/boss/api/v1"
	"github.com/crosbymichael/boss/config"
	"github.com/crosbymichael/boss/flux"
	"github.com/crosbymichael/boss/systemd"
	"github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

var (
	ErrNoID = errors.New("no id provided")

	empty = &types.Empty{}
)

func New(c *config.Config, client *containerd.Client, store config.ConfigStore) (*Agent, error) {
	register, err := c.GetRegister()
	if err != nil {
		return nil, err
	}
	return &Agent{
		c:        c,
		client:   client,
		store:    store,
		register: register,
	}, nil
}

type Agent struct {
	c        *config.Config
	client   *containerd.Client
	store    config.ConfigStore
	register v1.Register
}

func (a *Agent) Create(ctx context.Context, req *v1.CreateRequest) (*types.Empty, error) {
	ctx = relayContext(ctx)
	image, err := a.pull(ctx, req.Container.Image)
	if err != nil {
		return nil, err
	}
	container, err := a.client.NewContainer(ctx,
		req.Container.ID,
		flux.WithNewSnapshot(image),
		config.WithBossConfig(req.Container, image),
	)
	if err != nil {
		return nil, err
	}
	if err := a.store.Write(ctx, req.Container); err != nil {
		container.Delete(ctx, containerd.WithSnapshotCleanup)
		return nil, err
	}
	if err := systemd.Enable(ctx, container.ID()); err != nil {
		return nil, err
	}
	if err := systemd.Start(ctx, container.ID()); err != nil {
		return nil, err
	}
	return empty, nil
}

func (a *Agent) Delete(ctx context.Context, req *v1.DeleteRequest) (*types.Empty, error) {
	ctx = relayContext(ctx)
	id := req.ID
	if id == "" {
		return nil, ErrNoID
	}
	container, err := a.client.LoadContainer(ctx, id)
	if err != nil {
		return nil, errors.Wrap(err, "load container")
	}
	if err := systemd.Stop(ctx, id); err != nil {
		return nil, err
		return nil, errors.Wrap(err, "stop service")
	}
	if err := systemd.Disable(ctx, id); err != nil {
		return nil, errors.Wrap(err, "disable service")
	}
	config, err := config.GetConfig(ctx, container)
	if err != nil {
		return nil, errors.Wrap(err, "load config")
	}
	network, err := a.c.GetNetwork(config.Network)
	if err != nil {
		return nil, errors.Wrap(err, "get network")
	}
	if err := network.Remove(ctx, container); err != nil {
		return nil, err
	}
	for name := range config.Services {
		if err := a.register.Deregister(id, name); err != nil {
			logrus.WithError(err).Errorf("de-register %s-%s", id, name)
		}
	}
	return empty, container.Delete(ctx, containerd.WithSnapshotCleanup)
}

func (a *Agent) Get(ctx context.Context, req *v1.GetRequest) (*v1.GetResponse, error) {
	ctx = relayContext(ctx)
	id := req.ID
	if id == "" {
		return nil, ErrNoID
	}
	container, err := a.client.LoadContainer(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	config, err := config.GetConfig(ctx, container)
	if err != nil {
		return nil, err
	}
	return &v1.GetResponse{
		Container: config,
	}, nil
}

func (a *Agent) listContainer(ctx context.Context, c containerd.Container) (*v1.ListContainer, error) {
	info, err := c.Info(ctx)
	if err != nil {
		return nil, err
	}
	task, err := c.Task(ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return &v1.ListContainer{
				ID:     c.ID(),
				Image:  info.Image,
				Status: string(containerd.Stopped),
			}, nil
		}
		return nil, err
	}
	status, err := task.Status(ctx)
	if err != nil {
		return nil, err
	}
	stats, err := task.Metrics(ctx)
	if err != nil {
		return nil, err
	}
	d := info.Extensions[config.CurrentConfig]
	cfg, err := config.UnmarshalConfig(&d)
	if err != nil {
		return nil, err
	}
	v, err := typeurl.UnmarshalAny(stats.Data)
	if err != nil {
		return nil, err
	}
	var (
		cg      = v.(*cgroups.Metrics)
		cpu     = cg.CPU.Usage.Total
		memory  = float64(cg.Memory.Usage.Usage - cg.Memory.TotalCache)
		limit   = float64(cg.Memory.Usage.Limit)
		service = a.client.SnapshotService(info.Snapshotter)
	)
	usage, err := service.Usage(ctx, info.SnapshotKey)
	if err != nil {
		return nil, err
	}
	bindSizes, err := getBindSizes(cfg)
	if err != nil {
		return nil, err
	}
	return &v1.ListContainer{
		ID:          c.ID(),
		Image:       info.Image,
		Status:      string(status.Status),
		IP:          info.Labels[v1.IPLabel],
		Cpu:         cpu,
		MemoryUsage: memory,
		MemoryLimit: limit,
		PidUsage:    cg.Pids.Current,
		PidLimit:    cg.Pids.Limit,
		FsSize:      usage.Size + bindSizes,
	}, nil
}

func (a *Agent) List(ctx context.Context, req *v1.ListRequest) (*v1.ListResponse, error) {
	var resp v1.ListResponse
	ctx = relayContext(ctx)
	containers, err := a.client.Containers(ctx)
	if err != nil {
		return nil, err
	}
	for _, c := range containers {
		l, err := a.listContainer(ctx, c)
		if err != nil {
			resp.Containers = append(resp.Containers, &v1.ListContainer{
				ID:     c.ID(),
				Status: "list error",
			})
			logrus.WithError(err).Error("info container")
			continue
		}
		resp.Containers = append(resp.Containers, l)
	}
	return &resp, nil
}

func (a *Agent) Kill(ctx context.Context, req *v1.KillRequest) (*types.Empty, error) {
	ctx = relayContext(ctx)
	id := req.ID
	if id == "" {
		return nil, ErrNoID
	}
	container, err := a.client.LoadContainer(ctx, id)
	if err != nil {
		return nil, err
	}
	config, err := config.GetConfig(ctx, container)
	if err != nil {
		return nil, err
	}
	for name := range config.Services {
		if err := a.register.EnableMaintainance(id, name, "manual kill"); err != nil {
			logrus.WithError(err).Errorf("enable maintaince %s-%s", id, name)
		}
	}
	task, err := container.Task(ctx, nil)
	if err != nil {
		return nil, err
	}
	if err := task.Kill(ctx, unix.SIGTERM); err != nil {
		return nil, err
	}
	return empty, nil
}

func (a *Agent) Start(ctx context.Context, req *v1.StartRequest) (*types.Empty, error) {
	ctx = relayContext(ctx)
	id := req.ID
	if id == "" {
		return nil, ErrNoID
	}
	return empty, systemd.Start(ctx, req.ID)
}

func (a *Agent) Stop(ctx context.Context, req *v1.StopRequest) (*types.Empty, error) {
	ctx = relayContext(ctx)
	id := req.ID
	if id == "" {
		return nil, ErrNoID
	}
	return empty, systemd.Stop(ctx, req.ID)
}

func (a *Agent) Update(ctx context.Context, req *v1.UpdateRequest) (*v1.UpdateResponse, error) {
	ctx = relayContext(ctx)
	ctx, done, err := a.client.WithLease(ctx)
	if err != nil {
		return nil, err
	}
	defer done(ctx)
	container, err := a.client.LoadContainer(ctx, req.Container.ID)
	if err != nil {
		return nil, err
	}
	current, err := config.GetConfig(ctx, container)
	if err != nil {
		return nil, err
	}
	// set all current services into maintaince mode
	for name := range current.Services {
		if err := a.register.EnableMaintainance(container.ID(), name, "update container configuration"); err != nil {
			logrus.WithError(err).Errorf("enable maintaince %s-%s", container.ID(), name)
		}
	}
	var changes []change
	for name := range current.Services {
		if _, ok := req.Container.Services[name]; !ok {
			// if the new config does not have a service, deregister the old one
			changes = append(changes, &deregisterChange{
				register: a.register,
				name:     name,
			})
		}
	}
	changes = append(changes, &imageUpdateChange{
		ref:    req.Container.Image,
		client: a.client,
	})
	changes = append(changes, &configChange{
		client: a.client,
		c:      req.Container,
	})
	changes = append(changes, &filesChange{
		c:     req.Container,
		store: a.store,
	})
	err = pauseAndRun(ctx, container, func() error {
		for _, ch := range changes {
			if err := ch.update(ctx, container); err != nil {
				return err
			}
		}
		// bump the task to pickup the changes
		task, err := container.Task(ctx, nil)
		if err != nil {
			if errdefs.IsNotFound(err) {
				return nil
			}
			return err
		}
		return task.Kill(ctx, unix.SIGTERM)
	})
	if err != nil {
		return nil, err
	}
	return &v1.UpdateResponse{}, nil
}

func (a *Agent) Rollback(ctx context.Context, req *v1.RollbackRequest) (*v1.RollbackResponse, error) {
	ctx = relayContext(ctx)
	ctx, done, err := a.client.WithLease(ctx)
	if err != nil {
		return nil, err
	}
	defer done(ctx)
	container, err := a.client.LoadContainer(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	err = pauseAndRun(ctx, container, func() error {
		if err := container.Update(ctx, flux.WithRollback, config.WithRollback); err != nil {
			return err
		}
		task, err := container.Task(ctx, nil)
		if err != nil {
			return err
		}
		return task.Kill(ctx, unix.SIGTERM)
	})
	if err != nil {
		return nil, err
	}
	return &v1.RollbackResponse{}, nil
}

func (a *Agent) pull(ctx context.Context, ref string) (containerd.Image, error) {
	image, err := a.client.GetImage(ctx, ref)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return nil, err
		}
		if image, err = a.client.Pull(ctx, ref, containerd.WithPullUnpack); err != nil {
			return nil, err
		}
	}
	return image, nil
}

func getBindSizes(c *v1.Container) (size int64, _ error) {
	for _, m := range c.Mounts {
		f, err := os.Open(m.Source)
		if err != nil {
			return size, err
		}
		info, err := f.Stat()
		if err != nil {
			f.Close()
			return size, err
		}
		if info.IsDir() {
			f.Close()
			if err := filepath.Walk(m.Source, func(path string, wi os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if wi.IsDir() {
					return nil
				}
				size += wi.Size()
				return nil
			}); err != nil {
				return size, err
			}
			continue
		}
		size += info.Size()
		f.Close()
	}
	return size, nil
}

func relayContext(ctx context.Context) context.Context {
	return namespaces.WithNamespace(ctx, v1.DefaultNamespace)
}
