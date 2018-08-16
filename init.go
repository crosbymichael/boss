package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd/pkg/progress"
	"github.com/crosbymichael/boss/config"
	"github.com/crosbymichael/boss/system"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var initCommand = cli.Command{
	Name:  "init",
	Usage: "init boss on a system",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "undo",
			Usage: "remove all boss init steps from the system, :(",
		},
		cli.StringFlag{
			Name:  "step",
			Usage: "run a specific step by name",
		},
	},
	Action: func(clix *cli.Context) error {
		c, err := config.Load()
		if err != nil {
			return err
		}
		client, err := system.NewClient()
		if err != nil {
			return errors.Wrap(err, "unable to connect to containerd, it's the one thing you have to do...for now")
		}
		defer client.Close()
		var (
			undo  = clix.Bool("undo")
			r     = bufio.NewScanner(os.Stdin)
			steps = c.Steps()
		)
		steps = filter(steps, clix.String("step"))
		action := "install"
		if undo {
			action = "remove"
		}
		for _, s := range steps {
			if s.Name() == "dns" {
				fmt.Print("boss and consul are going to manage your server's DNS, is this ok? (y/n): ")
				r.Scan()
				yn := r.Text()
				if strings.Trim(yn, " \n") == "n" {
					fmt.Println("ok, aborting... :(")
					return nil
				}
			}
			fmt.Printf("%s -> %s\n", action, s.Name())
		}
		fmt.Printf("ready to %s..., continue? (y/n): ", action)
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
				name: s.Name(),
				i:    i,
			}
			stepProgress <- current
			cmu.Unlock()
			fn := s.Run
			if clix.Bool("undo") {
				fn = s.Remove
			}
			if err := fn(ctx, client, clix); err != nil {
				if clix.Bool("undo") {
					logrus.WithError(err).Warnf("execute %s", s.Name())
				} else {
					return errors.Wrapf(err, "execute %s", s.Name())
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

func filter(steps []config.Step, filter string) (o []config.Step) {
	if filter == "" {
		return steps
	}
	for _, s := range steps {
		if s.Name() == filter || s.Name() == config.RegisterName(filter) {
			o = append(o, s)
		}
	}
	return o
}
