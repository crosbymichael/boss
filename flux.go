package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/diff/apply"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/rootfs"
	"github.com/containerd/containerd/snapshots"
	"github.com/opencontainers/image-spec/identity"
)

const (
	gcRoot           = "containerd.io/gc.root"
	timestampFormat  = "01-02-2006-15:04:05"
	previousRevision = "boss.io/revision.previous"
	ImageLabel       = "boss.io/revision.image"
)

var ErrNoPreviousRevision = errors.New("no previous revision")

func newRevision(id string) *Revision {
	now := time.Now()
	return &Revision{
		Timestamp: now,
		Key:       fmt.Sprintf("boss.io.%s.%s", id, now.Format(timestampFormat)),
	}
}

type Revision struct {
	Timestamp time.Time
	Key       string
	mounts    []mount.Mount
}

func (r *Revision) Mounts() []mount.Mount {
	return r.mounts
}

func newFlux(client *containerd.Client) *Flux {
	return &Flux{
		client: client,
	}
}

type Flux struct {
	client *containerd.Client
}

// New creates a new initial revision from an image for a container
func (t *Flux) New(ctx context.Context, i containerd.Image, id string, previous string) (*Revision, error) {
	diffIDs, err := i.RootFS(ctx)
	if err != nil {
		return nil, err
	}
	var (
		parent = identity.ChainID(diffIDs).String()
		r      = newRevision(id)
	)
	labels := map[string]string{
		gcRoot:     r.Timestamp.Format(time.RFC3339),
		ImageLabel: i.Name(),
	}
	if previous != "" {
		labels[previousRevision] = previous
	}
	mounts, err := t.client.SnapshotService(containerd.DefaultSnapshotter).Prepare(ctx, r.Key, parent, snapshots.WithLabels(labels))
	if err != nil {
		return nil, err
	}
	r.mounts = mounts
	return r, nil
}

func (t *Flux) Save(ctx context.Context, container containerd.Container) (*Revision, error) {
	info, err := container.Info(ctx)
	if err != nil {
		return nil, err
	}
	// create a new snapshot from the container's image
	image, err := container.Image(ctx)
	if err != nil {
		return nil, err
	}
	snapshot, err := t.New(ctx, image, container.ID(), info.SnapshotKey)
	if err != nil {
		return nil, err
	}
	service := t.client.SnapshotService(info.Snapshotter)
	// create a diff from the existing snapshot
	diff, err := rootfs.CreateDiff(ctx, info.SnapshotKey, service, t.client.DiffService())
	if err != nil {
		return nil, err
	}
	applier := apply.NewFileSystemApplier(t.client.ContentStore())
	if _, err := applier.Apply(ctx, diff, snapshot.mounts); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func (t *Flux) Previous(ctx context.Context, container containerd.Container) (*Revision, error) {
	info, err := container.Info(ctx)
	if err != nil {
		return nil, err
	}
	service := t.client.SnapshotService(info.Snapshotter)
	sInfo, err := service.Stat(ctx, info.SnapshotKey)
	if err != nil {
		return nil, err
	}
	key := sInfo.Labels[previousRevision]
	if key == "" {
		return nil, ErrNoPreviousRevision
	}
	parts := strings.Split(key, ".")
	timestamp, err := time.Parse(timestampFormat, parts[len(parts)-1])
	if err != nil {
		return nil, err
	}
	return &Revision{
		Timestamp: timestamp,
		Key:       key,
	}, nil
}

func WithNewSnapshotFromImage(t *Flux, i containerd.Image) containerd.NewContainerOpts {
	return func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
		if c.Snapshotter == "" {
			c.Snapshotter = containerd.DefaultSnapshotter
		}
		r, err := t.New(ctx, i, c.ID, "")
		if err != nil {
			return err
		}
		c.SnapshotKey = r.Key
		c.Image = i.Name()
		return nil
	}
}

func WithRevision(r *Revision) containerd.UpdateContainerOpts {
	return func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
		c.SnapshotKey = r.Key
		return nil
	}
}
