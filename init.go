package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/pkg/progress"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

type step interface {
	name() string
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
		var (
			steps     []step
			hasConsul bool
			start     = time.Now()
		)
		if cfg.Consul != nil {
			hasConsul = true
			steps = append(steps, &consulStep{})
			if ips := clix.StringSlice("join"); len(ips) > 0 {
				steps = append(steps, &joinStep{ips: ips})
			}
		}
		if cfg.NodeMetrics != nil {
			steps = append(steps, &nodeMetricsStep{})
			if hasConsul {
				steps = append(steps, &registerStep{
					id: "node-exporter",
					tags: []string{
						"metrics",
					},
					port: 9100,
				})
			}
		}
		if cfg.Buildkit != nil {
			steps = append(steps, &buildkitStep{})
			if hasConsul {
				steps = append(steps, &registerStep{
					id:   "buildkit",
					port: 9000,
				})
			}
		}
		if cfg.CNI != nil {
			steps = append(steps, &cniStep{})
			if cfg.CNI.IPAM.Type == "dhcp" {
				steps = append(steps, &dhcpStep{})
			}
		}
		steps = append(steps, &agentStep{})
		if hasConsul {
			steps = append(steps, &registerStep{
				id: "containerd",
				tags: []string{
					"metrics",
				},
				port: 9200,
			})
		}
		var (
			fw    = progress.NewWriter(os.Stdout)
			total = float64(len(steps))
		)
		for i, s := range steps {
			if err := s.run(cfg.Context(), cfg.Client(), clix); err != nil {
				return errors.Wrapf(err, "install %s", s.name())
			}
			bar := progress.Bar(float64(i+1) / total)
			fmt.Fprintf(fw, "%s:\t%d/%d\t%40r\t\n", s.name(), i+1, int(total), bar)

			fmt.Fprintf(fw, "elapsed: %-4.1fs\t\n",
				time.Since(start).Seconds(),
			)
			fw.Flush()
		}
		return nil
	},
}
