package main

import (
	"context"
	"io"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cmd/ctr/commands/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/urfave/cli"
)

func getImage(ctx context.Context, client *containerd.Client, ref string, clix *cli.Context, out io.Writer, unpack bool) (containerd.Image, error) {
	image, err := client.GetImage(ctx, ref)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return nil, err
		}
		fc, err := content.NewFetchConfig(ctx, clix)
		if err != nil {
			return nil, err
		}
		fc.ProgressOutput = out
		if _, err := content.Fetch(ctx, client, ref, fc); err != nil {
			return nil, err
		}
		if image, err = client.GetImage(ctx, ref); err != nil {
			return nil, err
		}
		if unpack {
			if err := image.Unpack(ctx, containerd.DefaultSnapshotter); err != nil {
				return nil, err
			}
		}
	}
	return image, nil
}
