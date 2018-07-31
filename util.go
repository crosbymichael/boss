package main

import (
	"fmt"

	"github.com/hashicorp/consul/api"
)

func createRegistration(id, name, ip string, s Service) *api.AgentServiceRegistration {
	reg := &api.AgentServiceRegistration{
		ID:      id,
		Name:    name,
		Tags:    s.Labels,
		Port:    s.Port,
		Address: ip,
	}
	for _, c := range s.Checks {
		var check api.AgentServiceCheck
		check.Name = name
		if c.Interval != 0 {
			check.Interval = fmt.Sprintf("%ds", c.Interval)
		}
		if c.Timeout != 0 {
			check.Timeout = fmt.Sprintf("%ds", c.Timeout)
		}
		addr := fmt.Sprintf("%s:%d", ip, s.Port)
		switch c.Type {
		case HTTP:
			check.HTTP = addr
		case TCP:
			check.TCP = addr
		case GRPC:
			check.GRPC = addr
		}
		reg.Checks = append(reg.Checks, &check)
	}
	return reg
}
