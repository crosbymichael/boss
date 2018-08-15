package step

import (
	"context"
	"fmt"

	"github.com/containerd/containerd"
	"github.com/crosbymichael/boss/config"
	"github.com/crosbymichael/boss/util"
	"github.com/hashicorp/consul/api"
	"github.com/urfave/cli"
)

type Register struct {
	ID     string
	Port   int
	Tags   []string
	Config *config.Config
}

func (s *Register) Name() string {
	return RegisterName(s.ID)
}

func (s *Register) Run(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
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
	return consul.Agent().ServiceRegister(reg)
}

func (s *Register) Remove(ctx context.Context, client *containerd.Client, clix *cli.Context) error {
	consul, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return nil
	}
	consul.Agent().ServiceDeregister(s.ID)
	return nil
}
