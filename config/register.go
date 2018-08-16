package config

import (
	"context"
	"fmt"

	"github.com/containerd/containerd"
	"github.com/crosbymichael/boss/api/v1"
	"github.com/crosbymichael/boss/util"
	"github.com/hashicorp/consul/api"
	"github.com/urfave/cli"
)

type RegisterService struct {
	ID     string
	Port   int
	Tags   []string
	Config *Config
	Check  *v1.HealthCheck
}

func (s *RegisterService) Name() string {
	return RegisterName(s.ID)
}

func (s *RegisterService) Run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	ip, err := util.GetIP(s.Config.Iface)
	if err != nil {
		return err
	}
	consul, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return err
	}
	reg := &api.AgentServiceRegistration{
		ID:      fmt.Sprintf("%s-%s", s.ID, s.Config.ID),
		Name:    s.ID,
		Tags:    s.Tags,
		Port:    s.Port,
		Address: ip,
	}
	if s.Check != nil {
		var check api.AgentServiceCheck
		check.Name = s.ID
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
			url := ""
			check.HTTP = fmt.Sprintf("http://%s%s", addr, url)
			check.Method = s.Check.Method
		case "tcp":
			check.TCP = addr
		case "grpc":
			check.GRPC = addr
		}
		reg.Checks = append(reg.Checks, &check)
	}
	return consul.Agent().ServiceRegister(reg)
}

func (s *RegisterService) Remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	consul, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return nil
	}
	consul.Agent().ServiceDeregister(s.ID)
	return nil
}
