package main

import (
	"github.com/containerd/containerd/containers"
)

func ensureLabels(c *containers.Container) {
	if c.Labels == nil {
		c.Labels = make(map[string]string)
	}
}
