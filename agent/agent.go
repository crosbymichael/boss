package agent

import (
	"context"
	"errors"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/crosbymichael/boss/api/v1"
	"github.com/crosbymichael/boss/config"
	"github.com/crosbymichael/boss/flux"
	"github.com/crosbymichael/boss/systemd"
	"github.com/gogo/protobuf/types"
	"github.com/sirupsen/logrus"
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
	ctx = namespace(ctx)
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
	id := req.ID
	if id == "" {
		return nil, ErrNoID
	}
	container, err := a.client.LoadContainer(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := systemd.Stop(ctx, id); err != nil {
		return nil, err
	}
	if err := systemd.Disable(ctx, id); err != nil {
		return nil, err
	}
	config, err := config.GetConfig(ctx, container)
	if err != nil {
		return nil, err
	}
	network, err := a.c.GetNetwork(config.Network)
	if err != nil {
		return nil, err
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

func namespace(ctx context.Context) context.Context {
	return namespaces.WithNamespace(ctx, v1.DefaultNamespace)
}
