package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"

	"github.com/containerd/containerd/contrib/apparmor"
	"github.com/crosbymichael/boss/config"
	"github.com/crosbymichael/boss/monitor"
	"github.com/crosbymichael/boss/system"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli"
	"golang.org/x/sys/unix"
)

var agentCommand = cli.Command{
	Name:  "agent",
	Usage: "run the boss agent for restarting services",
	Before: func(clix *cli.Context) error {
		if err := system.Ready(cfg); err != nil {
			return err
		}
		f, err := os.Create(filepath.Join(config.Root, "resolv.conf"))
		if err != nil {
			return err
		}
		defer f.Close()
		// setup nameservers
		for _, ns := range cfg.Nameservers {
			if _, err := f.WriteString(fmt.Sprintf("nameserver %s\n", ns)); err != nil {
				return err
			}
		}
		// generate defalt profile
		return apparmor.WithDefaultProfile("boss")(nil, nil, nil, &specs.Spec{
			Process: &specs.Process{},
		})
	},
	Action: func(clix *cli.Context) error {
		signals := make(chan os.Signal, 64)
		signal.Notify(signals, unix.SIGTERM, unix.SIGINT)

		m := monitor.New(cfg)
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
		return m.Run()
	},
}
