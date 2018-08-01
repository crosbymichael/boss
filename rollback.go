package main

import (
	"context"
	"fmt"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/namespaces"
	"github.com/urfave/cli"
	"golang.org/x/sys/unix"
)

var rollbackCommand = cli.Command{
	Name:  "rollback",
	Usage: "rollback a container to a previous revision",
	Action: func(clix *cli.Context) error {
		ctx := namespaces.WithNamespace(context.Background(), clix.GlobalString("namespace"))
		client, err := containerd.New(
			defaults.DefaultAddress,
			containerd.WithDefaultRuntime("io.containerd.runc.v1"),
		)
		if err != nil {
			return err
		}
		defer client.Close()
		ctx, done, err := client.WithLease(ctx)
		if err != nil {
			return err
		}
		defer done(ctx)
		var (
			id   = clix.Args().First()
			flux = newFlux(client)
		)
		container, err := client.LoadContainer(ctx, id)
		if err != nil {
			return err
		}
		previous, err := flux.Previous(ctx, container)
		if err != nil {
			return err
		}
		info, err := container.Info(ctx)
		if err != nil {
			return err
		}
		ss := client.SnapshotService(info.Snapshotter)
		sInfo, err := ss.Stat(ctx, previous.Key)
		if err != nil {
			return err
		}
		snapshotImage, ok := sInfo.Labels[ImageLabel]
		if !ok {
			return fmt.Errorf("snapshot %s does not have a service image label", previous.Key)
		}
		if snapshotImage == "" {
			return fmt.Errorf("snapshot %s has an empty service image label", previous.Key)
		}
		image, err := getImage(ctx, client, snapshotImage, clix)
		if err != nil {
			return err
		}
		return pauseAndRun(ctx, id, client, func() error {
			if err := container.Update(ctx, withImage(image), WithRevision(previous)); err != nil {
				return err
			}
			task, err := container.Task(ctx, nil)
			if err != nil {
				return err
			}
			return task.Kill(ctx, unix.SIGTERM)
		})
	},
}
