package agent

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/crosbymichael/boss/api/v1"
	"github.com/crosbymichael/boss/config"
	"github.com/crosbymichael/boss/flux"
	"github.com/crosbymichael/boss/systemd"
	"github.com/gogo/protobuf/types"
)

var empty = &types.Empty{}

func New(c *config.Config, client *containerd.Client, store config.ConfigStore) (*Agent, error) {
	return &Agent{
		c:      c,
		client: client,
		store:  store,
	}, nil
}

type Agent struct {
	c      *config.Config
	client *containerd.Client
	store  config.ConfigStore
}

func (a *Agent) Close() error {
	return a.client.Close()
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
		req.Container.WithConfig(image),
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
