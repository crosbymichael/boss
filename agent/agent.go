package agent

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containerd/cgroups"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/snapshots"
	"github.com/containerd/typeurl"
	"github.com/crosbymichael/boss/api/v1"
	"github.com/crosbymichael/boss/config"
	"github.com/crosbymichael/boss/flux"
	"github.com/crosbymichael/boss/opts"
	"github.com/crosbymichael/boss/systemd"
	"github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

var (
	ErrNoID      = errors.New("no id provided")
	plainRemotes = make(map[string]bool)

	empty = &types.Empty{}
)

func New(c *config.Config, client *containerd.Client, store config.ConfigStore) (*Agent, error) {
	register, err := c.GetRegister()
	if err != nil {
		return nil, err
	}
	for _, r := range c.Agent.PlainRemotes {
		plainRemotes[r] = true
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
	image, err := a.client.Pull(ctx, req.Container.Image, containerd.WithPullUnpack, withPlainRemote(req.Container.Image))
	if err != nil {
		return nil, err
	}
	if _, err := a.client.LoadContainer(ctx, req.Container.ID); err == nil {
		if !req.Update {
			return nil, errors.Errorf("container %s already exists", req.Container.ID)
		}
		_, err = a.Update(ctx, &v1.UpdateRequest{
			Container: req.Container,
		})
		return empty, err
	}
	container, err := a.client.NewContainer(ctx,
		req.Container.ID,
		flux.WithNewSnapshot(image),
		opts.WithBossConfig(a.c.Agent.VolumeRoot, req.Container, image),
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
	config, err := opts.GetConfig(ctx, container)
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
	return empty, container.Delete(ctx, flux.WithRevisionCleanup)
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
	i, err := a.info(ctx, container)
	if err != nil {
		return nil, err
	}
	return &v1.GetResponse{
		Container: i,
	}, nil
}

func (a *Agent) info(ctx context.Context, c containerd.Container) (*v1.ContainerInfo, error) {
	info, err := c.Info(ctx)
	if err != nil {
		return nil, err
	}
	d := info.Extensions[opts.CurrentConfig]
	cfg, err := opts.UnmarshalConfig(&d)
	if err != nil {
		return nil, err
	}

	service := a.client.SnapshotService(info.Snapshotter)
	usage, err := service.Usage(ctx, info.SnapshotKey)
	if err != nil {
		return nil, err
	}
	var ss []*v1.Snapshot
	if err := service.Walk(ctx, func(ctx context.Context, si snapshots.Info) error {
		if si.Labels[flux.ContainerIDLabel] != c.ID() {
			return nil
		}
		usage, err := service.Usage(ctx, si.Name)
		if err != nil {
			return err
		}
		ss = append(ss, &v1.Snapshot{
			ID:       si.Name,
			Created:  si.Created,
			Previous: si.Labels[flux.PreviousLabel],
			FsSize:   usage.Size,
		})
		return nil
	}); err != nil {
		return nil, err
	}
	bindSizes, err := getBindSizes(cfg)
	if err != nil {
		return nil, err
	}
	task, err := c.Task(ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return &v1.ContainerInfo{
				ID:        c.ID(),
				Image:     info.Image,
				Status:    string(containerd.Stopped),
				FsSize:    usage.Size + bindSizes,
				Config:    cfg,
				Snapshots: ss,
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
	v, err := typeurl.UnmarshalAny(stats.Data)
	if err != nil {
		return nil, err
	}
	var (
		cg     = v.(*cgroups.Metrics)
		cpu    = cg.CPU.Usage.Total
		memory = float64(cg.Memory.Usage.Usage - cg.Memory.TotalCache)
		limit  = float64(cg.Memory.Usage.Limit)
	)
	return &v1.ContainerInfo{
		ID:          c.ID(),
		Image:       info.Image,
		Status:      string(status.Status),
		IP:          info.Labels[opts.IPLabel],
		Cpu:         cpu,
		MemoryUsage: memory,
		MemoryLimit: limit,
		PidUsage:    cg.Pids.Current,
		PidLimit:    cg.Pids.Limit,
		FsSize:      usage.Size + bindSizes,
		Config:      cfg,
		Snapshots:   ss,
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
		l, err := a.info(ctx, c)
		if err != nil {
			resp.Containers = append(resp.Containers, &v1.ContainerInfo{
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
	config, err := opts.GetConfig(ctx, container)
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
	current, err := opts.GetConfig(ctx, container)
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
		client:     a.client,
		c:          req.Container,
		volumeRoot: a.c.Agent.VolumeRoot,
	})
	changes = append(changes, &filesChange{
		c:     req.Container,
		store: a.store,
	})

	var wait <-chan containerd.ExitStatus
	// bump the task to pickup the changes
	task, err := container.Task(ctx, nil)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return nil, err
		}
	}
	if task != nil {
		if wait, err = task.Wait(ctx); err != nil {
			return nil, err
		}
	} else {
		c := make(chan containerd.ExitStatus)
		wait = c
		close(c)
	}
	err = pauseAndRun(ctx, container, func() error {
		for _, ch := range changes {
			if err := ch.update(ctx, container); err != nil {
				return err
			}
		}
		if task == nil {
			return nil
		}
		return task.Kill(ctx, unix.SIGTERM)
	})
	if err != nil {
		return nil, err
	}
	wctx, _ := context.WithTimeout(ctx, 10*time.Second)
	for {
		select {
		case <-wctx.Done():
			if task != nil {
				return &v1.UpdateResponse{}, task.Kill(ctx, unix.SIGKILL)
			}
			return nil, wctx.Err()
		case <-wait:
			return &v1.UpdateResponse{}, nil
		}
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
		if err := container.Update(ctx, flux.WithRollback, opts.WithRollback); err != nil {
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

func (a *Agent) PushBuild(ctx context.Context, req *v1.PushBuildRequest) (*types.Empty, error) {
	if req.Ref == "" {
		return nil, errors.New("no ref provided")
	}
	ctx = namespaces.WithNamespace(ctx, "buildkit")
	image, err := a.client.GetImage(ctx, req.Ref)
	if err != nil {
		return nil, err
	}
	return empty, a.client.Push(ctx, req.Ref, image.Target(), withPlainRemote(req.Ref))
}

func withPlainRemote(ref string) containerd.RemoteOpt {
	remote := strings.SplitN(ref, "/", 2)[0]
	return func(_ *containerd.Client, ctx *containerd.RemoteContext) error {
		ctx.Resolver = docker.NewResolver(docker.ResolverOptions{
			PlainHTTP: plainRemotes[remote],
			Client:    http.DefaultClient,
		})
		return nil
	}
}

func getBindSizes(c *v1.Container) (size int64, _ error) {
	for _, m := range c.Mounts {
		f, err := os.Open(m.Source)
		if err != nil {
			logrus.WithError(err).Warnf("unable to open bind for size %s", m.Source)
			continue
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
