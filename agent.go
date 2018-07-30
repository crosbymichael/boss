package main

import (
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/contrib/apparmor"
	"github.com/containerd/containerd/defaults"
	cni "github.com/containerd/go-cni"
	"github.com/hashicorp/consul/api"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli"
)

var agentCommand = cli.Command{
	Name:  "agent",
	Usage: "run the boss agent for restarting services",
	Flags: []cli.Flag{
		cli.DurationFlag{
			Name:  "interval,i",
			Usage: "set the interval to reconcile state",
			Value: 10 * time.Second,
		},
	},
	Action: func(clix *cli.Context) error {
		consul, err := api.NewClient(api.DefaultConfig())
		if err != nil {
			return err
		}
		// generate defalt profile
		if err := apparmor.WithDefaultProfile("boss")(nil, nil, nil, &specs.Spec{
			Process: &specs.Process{},
		}); err != nil {
			return err
		}
		client, err := containerd.New(
			defaults.DefaultAddress,
			containerd.WithDefaultRuntime("io.containerd.runc.v1"),
		)
		if err != nil {
			return err
		}
		defer client.Close()
		networking, err := cni.New(cni.WithDefaultConf)
		if err != nil {
			return err
		}
		m := &monitor{
			client:     client,
			networking: networking,
			consul:     consul,
		}
		if err := m.attach(); err != nil {
			return err
		}
		m.run(clix.Duration("interval"))
		return nil
	},
}
