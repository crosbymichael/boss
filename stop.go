package main

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/runtime/restart"
	"github.com/urfave/cli"
	"golang.org/x/sys/unix"
)

var logsCommand = cli.Command{
	Name:  "logs",
	Usage: "display service logs",
	Action: func(clix *cli.Context) error {
		f, err := os.Open(filepath.Join(clix.GlobalString("log-path"), clix.Args().First()))
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(os.Stdout, f)
		return err
	},
}

var startCommand = cli.Command{
	Name:  "start",
	Usage: "start an existing service",
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
		id := clix.Args().First()
		container, err := client.LoadContainer(ctx, id)
		if err != nil {
			return err
		}
		return container.Update(ctx, restart.WithStatus(containerd.Running))
	},
}

var stopCommand = cli.Command{
	Name:  "stop",
	Usage: "stop a running service",
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
		id := clix.Args().First()
		container, err := client.LoadContainer(ctx, id)
		if err != nil {
			return err
		}
		task, err := container.Task(ctx, nil)
		if err != nil {
			return err
		}
		wait, err := task.Wait(ctx)
		if err != nil {
			return err
		}
		if err := container.Update(ctx, restart.WithStatus(containerd.Stopped)); err != nil {
			return err
		}
		if err := task.Kill(ctx, unix.SIGTERM); err != nil {
			return err
		}
		<-wait
		if _, err := task.Delete(ctx); err != nil {
			return err
		}
		return nil
	},
}

var deleteCommand = cli.Command{
	Name:  "delete",
	Usage: "delete a service",
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
		id := clix.Args().First()
		container, err := client.LoadContainer(ctx, id)
		if err != nil {
			return err
		}
		return container.Delete(ctx, containerd.WithSnapshotCleanup)
	},
}
