package system

import (
	"fmt"
	"path/filepath"

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
	return c.client.Agent().EnableServiceMaintenance(serviceID(id, name), "created")
}

// Deregister sends the provided service registration to the local agent
func (c *Consul) Deregister(id, name string) error {
	return c.client.Agent().ServiceDeregister(serviceID(id, name))
}

// EnableMaintainance places the specific service in maintainace mode
func (c *Consul) EnableMaintainance(id, name, reason string) error {
	return c.client.Agent().EnableServiceMaintenance(serviceID(id, name), reason)
}

// DisableMaintainance removes the specific service out of maintainace mode
func (c *Consul) DisableMaintainance(id, name string) error {
	return c.client.Agent().DisableServiceMaintenance(serviceID(id, name))
}

func (c *Consul) registration(id, name, ip string, s config.Service) *api.AgentServiceRegistration {
	reg := &api.AgentServiceRegistration{
		ID:      serviceID(id, name),
		Name:    name,
		Tags:    s.Labels,
		Port:    s.Port,
		Address: ip,
	}
	if s.CheckType != "" {
		var check api.AgentServiceCheck
		check.Name = name
		if s.CheckInterval == 0 {
			s.CheckInterval = 10
		}
		check.Interval = fmt.Sprintf("%ds", s.CheckInterval)
		if s.CheckTimeout != 0 {
			check.Timeout = fmt.Sprintf("%ds", s.CheckTimeout)
		}
		addr := fmt.Sprintf("%s:%d", ip, s.Port)
		switch s.CheckType {
		case config.HTTP:
			url := s.URL
			if url != "" {
				url = filepath.Join("/", url)
			}
			check.HTTP = fmt.Sprintf("http://%s%s", addr, url)
			check.Method = s.CheckMethod
		case config.TCP:
			check.TCP = addr
		case config.GRPC:
			check.GRPC = addr
		}
		reg.Checks = append(reg.Checks, &check)
	}
	return reg
}

func serviceID(id, name string) string {
	return fmt.Sprintf("%s-%s", id, name)
}
