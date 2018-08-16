package config

import "github.com/crosbymichael/boss/api/v1"

type nullRegister struct {
}

// Register sends the provided service registration to the local agent
func (c *nullRegister) Register(id, name, ip string, s *v1.Service) error {
	return nil
}

// Deregister sends the provided service registration to the local agent
func (c *nullRegister) Deregister(_, _ string) error {
	return nil
}

// EnableMaintainance places the specific service in maintainace mode
func (c *nullRegister) EnableMaintainance(_, _, _ string) error {
	return nil
}

// DisableMaintainance removes the specific service out of maintainace mode
func (c *nullRegister) DisableMaintainance(_, _ string) error {
	return nil
}
