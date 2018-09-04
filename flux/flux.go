package flux

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/diff/apply"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/rootfs"
	"github.com/containerd/containerd/snapshots"
	"github.com/opencontainers/image-spec/identity"
	"github.com/pkg/errors"
)

const (
	gcRoot           = "containerd.io/gc.root"
	timestampFormat  = "01-02-2006-15:04:05"
	PreviousLabel    = "boss.io/revision.previous"
	ImageLabel       = "boss.io/revision.image"
	ContainerIDLabel = "boss.io/revision.container"
)

var ErrNoPreviousRevision = errors.New("no previous revision")

// WithNewSnapshot creates a new snapshot managed by flux
func WithNewSnapshot(i containerd.Image) containerd.NewContainerOpts {
	return func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
		if c.Snapshotter == "" {
			c.Snapshotter = containerd.DefaultSnapshotter
		}
		r, err := create(ctx, client, i, c, c.ID, "")
		if err != nil {
			return err
		}
		c.SnapshotKey = r.Key
		c.Image = i.Name()
		return nil
	}
}

// WithUpgrade upgrades an existing container's image to a new one
func WithUpgrade(i containerd.Image) containerd.UpdateContainerOpts {
	return func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
		revision, err := save(ctx, client, i, c)
		if err != nil {
			return err
		}
		c.Image = i.Name()
		c.SnapshotKey = revision.Key
		return nil
	}
}

// WithRollback rolls back to the previous container's revision
func WithRollback(ctx context.Context, client *containerd.Client, c *containers.Container) error {
	prev, err := previous(ctx, client, c)
	if err != nil {
		return err
	}
	ss := client.SnapshotService(c.Snapshotter)
	sInfo, err := ss.Stat(ctx, prev.Key)
	if err != nil {
		return err
	}
	snapshotImage, ok := sInfo.Labels[ImageLabel]
	if !ok {
		return fmt.Errorf("snapshot %s does not have a service image label", prev.Key)
	}
	if snapshotImage == "" {
		return fmt.Errorf("snapshot %s has an empty service image label", prev.Key)
	}
	c.Image = snapshotImage
	c.SnapshotKey = prev.Key
	return nil
}

// WithRevisionCleanup cleans up all revisions for a container
func WithRevisionCleanup(ctx context.Context, client *containerd.Client, c containers.Container) error {
	if c.Snapshotter == "" {
		return errors.Wrapf(errdefs.ErrInvalidArgument, "container.Snapshotter must be set to cleanup rootfs snapshot")
	}
	var (
		keys []string
		ss   = client.SnapshotService(c.Snapshotter)
	)
	if err := ss.Walk(ctx, func(ctx context.Context, si snapshots.Info) error {
		if si.Labels[ContainerIDLabel] == c.ID {
			keys = append(keys, si.Name)
		}
		return nil
	}); err != nil {
		return err
	}
	for _, key := range keys {
		if err := client.SnapshotService(c.Snapshotter).Remove(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

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

func create(ctx context.Context, client *containerd.Client, i containerd.Image, c *containers.Container, id string, previous string) (*Revision, error) {
	diffIDs, err := i.RootFS(ctx)
	if err != nil {
		return nil, err
	}
	var (
		parent = identity.ChainID(diffIDs).String()
		r      = newRevision(id)
	)
	labels := map[string]string{
		gcRoot:           r.Timestamp.Format(time.RFC3339),
		ImageLabel:       i.Name(),
		ContainerIDLabel: id,
	}
	if previous != "" {
		labels[PreviousLabel] = previous
	}
	mounts, err := client.SnapshotService(c.Snapshotter).Prepare(ctx, r.Key, parent, snapshots.WithLabels(labels))
	if err != nil {
		return nil, err
	}
	r.mounts = mounts
	return r, nil
}

func save(ctx context.Context, client *containerd.Client, updatedImage containerd.Image, c *containers.Container) (*Revision, error) {
	snapshot, err := create(ctx, client, updatedImage, c, c.ID, c.SnapshotKey)
	if err != nil {
		return nil, err
	}
	service := client.SnapshotService(c.Snapshotter)
	// create a diff from the existing snapshot
	diff, err := rootfs.CreateDiff(ctx, c.SnapshotKey, service, client.DiffService())
	if err != nil {
		return nil, err
	}
	applier := apply.NewFileSystemApplier(client.ContentStore())
	if _, err := applier.Apply(ctx, diff, snapshot.mounts); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func previous(ctx context.Context, client *containerd.Client, c *containers.Container) (*Revision, error) {
	service := client.SnapshotService(c.Snapshotter)
	sInfo, err := service.Stat(ctx, c.SnapshotKey)
	if err != nil {
		return nil, err
	}
	key := sInfo.Labels[PreviousLabel]
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
