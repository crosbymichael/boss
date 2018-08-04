package main

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cmd/ctr/commands/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/urfave/cli"
)

func getImage(ctx context.Context, client *containerd.Client, ref string, clix *cli.Context) (containerd.Image, error) {
	image, err := client.GetImage(ctx, ref)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return nil, err
		}
		if _, err := content.Fetch(ctx, client, ref, clix); err != nil {
			return nil, err
		}
		if image, err = client.GetImage(ctx, ref); err != nil {
			return nil, err
		}
		if err := image.Unpack(ctx, containerd.DefaultSnapshotter); err != nil {
			return nil, err
		}
	}
	return image, nil
}
