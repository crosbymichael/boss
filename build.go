// stole from buildkit, tonis said it was ok
package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/containerd/console"
	"github.com/crosbymichael/boss/api/v1"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/util/progress/progressui"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
)

const BossDefaultBuildkitAddress = "127.0.0.1:9500"

//  sudo buildctl build --frontend=dockerfile.v0 --local context=. --local dockerfile=. --exporter=image --exporter-opt name=registry2
var buildCommand = cli.Command{
	Name:  "build",
	Usage: "build",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "name",
			Usage: "Name of the image to create",
		},
		cli.StringFlag{
			Name:  "export-cache",
			Usage: "Reference to export build cache to",
		},
		cli.StringSliceFlag{
			Name:  "export-cache-opt",
			Usage: "Define custom options for cache exporting",
		},
		cli.StringSliceFlag{
			Name:  "import-cache",
			Usage: "Reference to import build cache from",
		},
		cli.BoolFlag{
			Name:  "push",
			Usage: "push the resulting image",
		},
		cli.StringFlag{
			Name:  "dockerfile,d",
			Usage: "set the specific dockerfile",
			Value: ".",
		},
		cli.StringFlag{
			Name:  "context",
			Usage: "set the specific context path",
			Value: ".",
		},
		cli.StringFlag{
			Name:   "address",
			Usage:  "buildkitd address",
			Value:  BossDefaultBuildkitAddress,
			EnvVar: "BOSS_BUILDKIT",
		},
		cli.BoolFlag{
			Name:   "no-export",
			Usage:  "don't export the build",
			Hidden: true,
		},
		cli.StringFlag{
			Name:  "exporter",
			Usage: "set the buildkit exporter",
			Value: "image",
		},
		cli.StringSliceFlag{
			Name:  "build-arg",
			Usage: "set build args",
			Value: &cli.StringSlice{},
		},
	},
	Subcommands: []cli.Command{
		pushBuildCommand,
	},
	Action: func(clix *cli.Context) error {
		if err := build(clix); err != nil {
			return err
		}
		if !clix.Bool("push") {
			return nil
		}
		ref := clix.String("name")
		agent, err := Agent(clix)
		if err != nil {
			return err
		}
		defer agent.Close()
		_, err = agent.PushBuild(Context(), &v1.PushBuildRequest{
			Ref: ref,
		})
		return err
	},
}

var pushBuildCommand = cli.Command{
	Name:  "push",
	Usage: "push a build using the agent",
	Action: func(clix *cli.Context) error {
		agent, err := Agent(clix)
		if err != nil {
			return err
		}
		defer agent.Close()
		_, err = agent.Push(Context(), &v1.PushRequest{
			Ref:   clix.Args().First(),
			Build: true,
		})
		return err
	},
}

func read(r io.Reader, clicontext *cli.Context) (*llb.Definition, error) {
	def, err := llb.ReadFrom(r)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse input")
	}
	return def, nil
}

func build(clicontext *cli.Context) error {
	c, err := resolveClient(clicontext)
	if err != nil {
		return err
	}

	ch := make(chan *client.SolveStatus)
	eg, ctx := errgroup.WithContext(commandContext(clicontext))

	exporter := clicontext.String("exporter")
	if clicontext.Bool("no-export") {
		exporter = ""
	}
	atters := make(map[string]string)

	for _, a := range clicontext.StringSlice("build-arg") {
		kv := strings.SplitN(a, "=", 2)
		if len(kv) != 2 {
			return errors.Errorf("invalid build-arg value %s", a)
		}
		atters["build-arg:"+kv[0]] = kv[1]
	}

	solveOpt := client.SolveOpt{
		Exporter:      exporter,
		ExporterAttrs: make(map[string]string),
		// LocalDirs is set later
		Frontend:      "dockerfile.v0",
		FrontendAttrs: atters,
		ExportCache:   clicontext.String("export-cache"),
		ImportCache:   clicontext.StringSlice("import-cache"),
		Session:       []session.Attachable{authprovider.NewDockerAuthProvider()},
	}
	if !clicontext.Bool("no-export") {
		name := clicontext.String("name")
		if solveOpt.Exporter == "local" {
			solveOpt.ExporterAttrs["output"] = "."
		} else {
			if name == "" {
				return errors.New("name is required when exporting")
			}
			solveOpt.ExporterAttrs["name"] = name
		}
		solveOpt.ExporterOutput, solveOpt.ExporterOutputDir, err = resolveExporterOutput(solveOpt.Exporter, solveOpt.ExporterAttrs["output"])
		if err != nil {
			return errors.Wrap(err, "invalid exporter-opt: output")
		}
		if solveOpt.ExporterOutput != nil || solveOpt.ExporterOutputDir != "" {
			delete(solveOpt.ExporterAttrs, "output")
		}
	}

	exportCacheAttrs, err := attrMap(clicontext.StringSlice("export-cache-opt")...)
	if err != nil {
		return errors.Wrap(err, "invalid export-cache-opt")
	}
	if len(exportCacheAttrs) == 0 {
		exportCacheAttrs = map[string]string{"mode": "min"}
	}
	solveOpt.ExportCacheAttrs = exportCacheAttrs

	solveOpt.LocalDirs, err = attrMap(
		fmt.Sprintf("context=%s", clicontext.String("context")),
		fmt.Sprintf("dockerfile=%s", clicontext.String("dockerfile")),
	)
	if err != nil {
		return errors.Wrap(err, "invalid local")
	}

	var def *llb.Definition
	eg.Go(func() error {
		resp, err := c.Solve(ctx, def, solveOpt, ch)
		if err != nil {
			return err
		}
		for k, v := range resp.ExporterResponse {
			logrus.Debugf("solve response: %s=%s", k, v)
		}
		return err
	})

	displayCh := ch

	eg.Go(func() error {
		var c console.Console
		if cf, err := console.ConsoleFromFile(os.Stderr); err == nil {
			c = cf
		}
		// not using shared context to not disrupt display but let is finish reporting errors
		return progressui.DisplaySolveStatus(context.TODO(), "", c, os.Stdout, displayCh)
	})

	return eg.Wait()
}

func attrMap(sl ...string) (map[string]string, error) {
	m := map[string]string{}
	for _, v := range sl {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) != 2 {
			return nil, errors.Errorf("invalid value %s", v)
		}
		m[parts[0]] = parts[1]
	}
	return m, nil
}

// resolveExporterOutput returns at most either one of io.WriteCloser (single file) or a string (directory path).
func resolveExporterOutput(exporter, output string) (io.WriteCloser, string, error) {
	switch exporter {
	case client.ExporterLocal:
		if output == "" {
			return nil, "", errors.New("output directory is required for local exporter")
		}
		return nil, output, nil
	case client.ExporterOCI, client.ExporterDocker:
		if output != "" {
			fi, err := os.Stat(output)
			if err != nil && !os.IsNotExist(err) {
				return nil, "", errors.Wrapf(err, "invalid destination file: %s", output)
			}
			if err == nil && fi.IsDir() {
				return nil, "", errors.Errorf("destination file is a directory")
			}
			w, err := os.Create(output)
			return w, "", err
		}
		// if no output file is specified, use stdout
		if _, err := console.ConsoleFromFile(os.Stdout); err == nil {
			return nil, "", errors.Errorf("output file is required for %s exporter. refusing to write to console", exporter)
		}
		return os.Stdout, "", nil
	default: // e.g. client.ExporterImage
		if output != "" {
			return nil, "", errors.Errorf("output %s is not supported by %s exporter", output, exporter)
		}
		return nil, "", nil
	}
}

func commandContext(c *cli.Context) context.Context {
	return context.Background()
}

func resolveClient(c *cli.Context) (*client.Client, error) {
	opts := []client.ClientOpt{client.WithBlock()}
	ctx := commandContext(c)
	if span := opentracing.SpanFromContext(ctx); span != nil {
		opts = append(opts, client.WithTracer(span.Tracer()))
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return client.New(ctx, buildkitProto(c.String("address")), opts...)
}

func buildkitProto(s string) string {
	if strings.HasPrefix(s, "unix://") || strings.HasPrefix(s, "tcp://") {
		return s
	}
	if _, _, err := net.SplitHostPort(s); err == nil {
		return fmt.Sprintf("tcp://%s", s)
	}
	return fmt.Sprintf("unix://%s", s)
}
