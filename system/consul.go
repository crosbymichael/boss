package system

import (
	"fmt"

	"github.com/crosbymichael/boss/config"
	"github.com/hashicorp/consul/api"
)

// Consul is a connection to the local consul agent
type Consul struct {
	client *api.Client
}

// Register sends the provided service registration to the local agent
func (c *Consul) Register(id, name, ip string, s config.Service) error {
	reg := c.registration(id, name, ip, s)
	if err := c.client.Agent().ServiceRegister(reg); err != nil {
		return err
	}
	return c.client.Agent().EnableServiceMaintenance(id, "created")
}

// Deregister sends the provided service registration to the local agent
func (c *Consul) Deregister(id string) error {
	return c.client.Agent().ServiceDeregister(id)
}

// EnableMaintainance places the specific service in maintainace mode
func (c *Consul) EnableMaintainance(id, reason string) error {
	return c.client.Agent().EnableServiceMaintenance(id, reason)
}

// DisableMaintainance removes the specific service out of maintainace mode
func (c *Consul) DisableMaintainance(id string) error {
	return c.client.Agent().DisableServiceMaintenance(id)
}

func (c *Consul) registration(id, name, ip string, s config.Service) *api.AgentServiceRegistration {
	reg := &api.AgentServiceRegistration{
		ID:      id,
		Name:    name,
		Tags:    s.Labels,
		Port:    s.Port,
		Address: ip,
	}
	if s.CheckType != "" {
		var check api.AgentServiceCheck
		check.Name = name
		if s.CheckInterval != 0 {
			check.Interval = fmt.Sprintf("%ds", s.CheckInterval)
		}
		if s.CheckTimeout != 0 {
			check.Timeout = fmt.Sprintf("%ds", s.CheckTimeout)
		}
		addr := fmt.Sprintf("%s:%d", ip, s.Port)
		switch s.CheckType {
		case config.HTTP:
			check.HTTP = addr
		case config.TCP:
			check.TCP = addr
		case config.GRPC:
			check.GRPC = addr
		}
		reg.Checks = append(reg.Checks, &check)
	}
	return reg
}
