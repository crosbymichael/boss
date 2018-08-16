package consulregister

import (
	"fmt"
	"path/filepath"

	"github.com/crosbymichael/boss/api/v1"
	"github.com/hashicorp/consul/api"
)

func New(client *api.Client) *Consul {
	return &Consul{
		client: client,
	}
}

// Consul is a connection to the local consul agent
type Consul struct {
	client *api.Client
}

// Register sends the provided service registration to the local agent
func (c *Consul) Register(id, name, ip string, s *v1.Service) error {
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

func (c *Consul) registration(id, name, ip string, s *v1.Service) *api.AgentServiceRegistration {
	reg := &api.AgentServiceRegistration{
		ID:      serviceID(id, name),
		Name:    name,
		Tags:    s.Labels,
		Port:    int(s.Port),
		Address: ip,
	}
	if s.Check != nil {
		var check api.AgentServiceCheck
		check.Name = name
		if s.Check.Interval == 0 {
			s.Check.Interval = 10
		}
		check.Interval = fmt.Sprintf("%ds", s.Check.Interval)
		if s.Check.Timeout != 0 {
			check.Timeout = fmt.Sprintf("%ds", s.Check.Timeout)
		}
		addr := fmt.Sprintf("%s:%d", ip, s.Port)
		switch s.Check.Type {
		case "http":
			url := s.Url
			if url != "" {
				url = filepath.Join("/", url)
			}
			check.HTTP = fmt.Sprintf("http://%s%s", addr, url)
			check.Method = s.Check.Method
		case "tcp":
			check.TCP = addr
		case "grpc":
			check.GRPC = addr
		}
		reg.Checks = append(reg.Checks, &check)
	}
	return reg
}

func serviceID(id, name string) string {
	return fmt.Sprintf("%s-%s", id, name)
}
