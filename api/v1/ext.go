package v1

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
)

const (
	LastConfig = "io.boss/container.last"
	IPLabel    = "io/boss/container.ip"
)

// WithIP sets the ip on the container
func WithIP(ip string) containerd.UpdateContainerOpts {
	return func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
		if c.Labels == nil {
			c.Labels = make(map[string]string)
		}
		c.Labels[IPLabel] = ip
		return nil
	}
}
