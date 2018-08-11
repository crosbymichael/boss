package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd/pkg/progress"
	"github.com/crosbymichael/boss/system"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var initCommand = cli.Command{
	Name:  "init",
	Usage: "init boss on a system",
	Flags: []cli.Flag{
		cli.StringSliceFlag{
			Name:  "join",
			Usage: "list of consul servers to join",
			Value: &cli.StringSlice{},
		},
		cli.BoolFlag{
			Name:  "undo",
			Usage: "remove all boss init steps from the system, goodbye",
		},
	},
	Action: func(clix *cli.Context) error {
		var (
			hasConsul bool
			steps     []step
			undo      = clix.Bool("undo")
		)
		c, err := system.Load()
		if err != nil {
			return err
		}
		client, err := system.NewClient()
		if err != nil {
			return errors.Wrap(err, "unable to connect to containerd, it's the one thing you have to do...for now")
		}
		defer client.Close()
		steps = append(steps, &mkdirRoot{}, &bossUnit{}, &timezoneStep{config: c})
		if c.Consul != nil {
			hasConsul = true
			steps = append(steps, &consulStep{config: c})
			if ips := clix.StringSlice("join"); len(ips) > 0 {
				steps = append(steps, &joinStep{ips: ips})
			}
		}
		if c.NodeMetrics != nil {
			steps = append(steps, &nodeMetricsStep{config: c})
			if hasConsul {
				steps = append(steps, &registerStep{
					config: c,
					id:     "node-exporter",
					tags: []string{
						"metrics",
					},
					port: 9100,
				})
			}
		}
		if c.Buildkit != nil {
			steps = append(steps, &buildkitStep{config: c})
			if hasConsul {
				steps = append(steps, &registerStep{
					config: c,
					id:     "buildkit",
					port:   9000,
				})
			}
		}
		if c.CNI != nil {
			steps = append(steps, &cniStep{config: c})
			if c.CNI.IPAM.Type == "dhcp" {
				steps = append(steps, &dhcpStep{})
			}
		}
		if hasConsul {
			steps = append(steps, &registerStep{
				config: c,
				id:     "containerd",
				tags: []string{
					"metrics",
				},
				port: 9200,
			})
			steps = append(steps, &resolvedStep{ID: c.ID})
		}
		r := bufio.NewScanner(os.Stdin)

		action := "install"
		if undo {
			action = "remove"
		}
		for _, s := range steps {
			fmt.Printf("%s -> %s\n", action, s.name())
		}
		fmt.Printf("ready to %s, continue? (y/n): ", action)
		r.Scan()
		yn := r.Text()
		if strings.Trim(yn, " \n") == "n" {
			fmt.Println("ok, aborting... :(")
			return nil
		}
		var (
			cmu          sync.Mutex
			pwg          sync.WaitGroup
			start        = time.Now()
			fw           = progress.NewWriter(os.Stderr)
			total        = float64(len(steps))
			ctx          = system.Context()
			stepProgress = make(chan output, 10)
			current      = output{
				i:    0,
				name: "init",
			}
		)
		pwg.Add(1)
		go func() {
			defer pwg.Done()
			for s := range stepProgress {
				bar := progress.Bar(float64(s.i+1) / total)
				fmt.Fprintf(fw, "%s:\t%d/%d\t%40r\t\n", s.name, s.i+1, int(total), bar)

				fmt.Fprintf(fw, "elapsed: %-4.1fs\t\n",
					time.Since(start).Seconds(),
				)
				fw.Flush()
			}
		}()
		ticker := time.NewTicker(100 * time.Millisecond)
		go func() {
			for range ticker.C {
				cmu.Lock()
				stepProgress <- current
				cmu.Unlock()
			}
		}()
		for i, s := range steps {
			cmu.Lock()
			current = output{
				name: s.name(),
				i:    i,
			}
			stepProgress <- current
			cmu.Unlock()
			fn := s.run
			if clix.Bool("undo") {
				fn = s.remove
			}
			if err := fn(ctx, client, clix); err != nil {
				if clix.Bool("undo") {
					logrus.WithError(err).Warnf("execute %s", s.name())
				} else {
					return errors.Wrapf(err, "execute %s", s.name())
				}
			}
		}
		ticker.Stop()
		close(stepProgress)
		pwg.Wait()
		if clix.Bool("undo") {
			fmt.Println("boss removed and out of your way")
			return nil
		}
		fmt.Println("boss intitalized and ready for use, have fun!")
		return nil
	},
}

type output struct {
	i    int
	name string
}
