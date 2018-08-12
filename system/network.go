package system

import (
	"context"

	"github.com/containerd/containerd"
)

type host struct {
	ip string
}

func (n *host) Create(_ context.Context, _ containerd.Container) (string, error) {
	return n.ip, nil
}

func (n *host) Remove(_ context.Context, _ containerd.Container) error {
	return nil
}

type none struct {
}

func (n *none) Create(_ context.Context, _ containerd.Container) (string, error) {
	return "", nil
}

func (n *none) Remove(_ context.Context, _ containerd.Container) error {
	return nil
}
