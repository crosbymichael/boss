package main

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/urfave/cli"
)

type step interface {
	run(context.Context, *containerd.Client, *cli.Context) error
}

var initCommand = cli.Command{
	Name:  "init",
	Usage: "init boss on a system",
	Flags: []cli.Flag{
		cli.StringSliceFlag{
			Name:  "join",
			Usage: "list of consul servers to join",
			Value: &cli.StringSlice{},
		},
	},
	Action: func(clix *cli.Context) error {
		var steps []step
		if cfg.NodeMetrics != nil {
			steps = append(steps, &nodeMetricsStep{})
			// TODO: register system services
		}
		if cfg.Buildkit != nil {
			steps = append(steps, &buildkitStep{})
		}
		if cfg.Consul != nil {
			steps = append(steps, &consulStep{})
			if ips := clix.StringSlice("join"); len(ips) > 0 {
				steps = append(steps, &joinStep{ips: ips})
			}
		}
		if cfg.CNI != nil {
			steps = append(steps, &cniStep{})
			if cfg.CNI.IPAM.Type == "dhcp" {
				steps = append(steps, &dhcpStep{})
			}
		}
		steps = append(steps, &agentStep{})
		for _, s := range steps {
			if err := s.run(cfg.Context(), cfg.Client(), clix); err != nil {
				return err
			}
		}
		return nil
	},
}
