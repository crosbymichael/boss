package main

// Register is an object that registers and manages service information in its backend
type Register interface {
	Register(id, name, ip string, s Service) error
	Deregister(id string) error
	EnableMaintainance(id, msg string) error
	DisableMaintainance(id string) error
}

type nullRegister struct {
}

// Register sends the provided service registration to the local agent
func (c *nullRegister) Register(id, name, ip string, s Service) error {
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
