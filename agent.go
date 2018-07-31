package main

import (
	"os"
	"os/signal"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/contrib/apparmor"
	"github.com/containerd/containerd/defaults"
	cni "github.com/containerd/go-cni"
	"github.com/hashicorp/consul/api"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli"
	"golang.org/x/sys/unix"
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
		signals := make(chan os.Signal, 64)
		signal.Notify(signals, unix.SIGTERM)

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
			shutdownCh: make(chan struct{}, 1),
		}
		go func() {
			for range signals {
				m.shutdown()
			}
		}()
		if err := m.attach(); err != nil {
			return err
		}
		m.run(clix.Duration("interval"))
		return nil
	},
}
