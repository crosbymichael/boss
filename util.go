package main

import (
	"github.com/containerd/containerd/containers"
	"github.com/crosbymichael/boss/config"
)

type nullRegister struct {
}

// Register sends the provided service registration to the local agent
func (c *nullRegister) Register(id, name, ip string, s config.Service) error {
	return nil
}

// Deregister sends the provided service registration to the local agent
func (c *nullRegister) Deregister(id string) error {
	return nil
}

// EnableMaintainance places the specific service in maintainace mode
func (c *nullRegister) EnableMaintainance(id, reason string) error {
	return nil
}

// DisableMaintainance removes the specific service out of maintainace mode
func (c *nullRegister) DisableMaintainance(id string) error {
	return nil
}

func ensureLabels(c *containers.Container) {
	if c.Labels == nil {
		c.Labels = make(map[string]string)
	}
}
