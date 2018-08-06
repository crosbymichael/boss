package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/contrib/apparmor"
	"github.com/containerd/containerd/defaults"
	gocni "github.com/containerd/go-cni"
	"github.com/crosbymichael/boss/config"
	"github.com/crosbymichael/boss/monitor"
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
		cli.StringSliceFlag{
			Name:  "nameservers,n",
			Usage: "set the boss nameservers",
			Value: &cli.StringSlice{
				"8.8.8.8",
				"8.8.4.4",
			},
		},
	},
	Before: func(clix *cli.Context) error {
		f, err := os.Create(filepath.Join(config.Root, "resolv.conf"))
		if err != nil {
			return err
		}
		defer f.Close()
		for _, ns := range clix.StringSlice("nameservers") {
			if _, err := f.WriteString(fmt.Sprintf("nameserver %s\n", ns)); err != nil {
				return err
			}
		}
		return nil
	},
	Action: func(clix *cli.Context) error {
		signals := make(chan os.Signal, 64)
		signal.Notify(signals, unix.SIGTERM, unix.SIGINT)

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

		networks := make(map[config.NetworkType]monitor.Network)
		networks[config.Host] = &host{}
		networks[config.None] = &none{}
		if networking, err := gocni.New(gocni.WithPluginDir([]string{"/opt/containerd/bin"}), gocni.WithDefaultConf); err == nil {
			networks[config.CNI] = &cni{network: networking}
		}

		m := monitor.New(client, register, networks)
		var once sync.Once
		go func() {
			for s := range signals {
				switch s {
				case unix.SIGTERM:
					once.Do(m.Shutdown)
				case unix.SIGINT:
					once.Do(func() {
						m.Stop()
					})
				}
			}
		}()
		if err := m.Attach(); err != nil {
			return err
		}
		m.Run(clix.Duration("interval"))
		return nil
	},
}
