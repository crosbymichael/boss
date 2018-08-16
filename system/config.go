package system

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/namespaces"
	"github.com/crosbymichael/boss/api/v1"
)

// Context returns a new context to be used by boss
func Context() context.Context {
	return namespaces.WithNamespace(context.Background(), v1.DefaultNamespace)
}

// NewClient returns a new containerd client
func NewClient() (*containerd.Client, error) {
	return containerd.New(
		defaults.DefaultAddress,
		containerd.WithDefaultRuntime(v1.DefaultRuntime),
		containerd.WithDefaultNamespace(v1.DefaultNamespace),
	)
}
