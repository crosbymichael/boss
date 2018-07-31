// stole from buildkit, tonis said it was ok
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/containerd/console"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/appdefaults"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/opencontainers/go-digest"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
)

//  sudo buildctl build --frontend=dockerfile.v0 --local context=. --local dockerfile=. --exporter=image --exporter-opt name=registry2
var buildCommand = cli.Command{
	Name:   "build",
	Usage:  "build",
	Action: build,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "name",
			Usage: "Name of the image to create",
		},
		cli.BoolFlag{
			Name:  "no-cache",
			Usage: "Disable cache for all the vertices. Frontend is not supported.",
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
	},
}

func read(r io.Reader, clicontext *cli.Context) (*llb.Definition, error) {
	def, err := llb.ReadFrom(r)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse input")
	}
	if clicontext.Bool("no-cache") {
		for _, dt := range def.Def {
			var op pb.Op
			if err := (&op).Unmarshal(dt); err != nil {
				return nil, errors.Wrap(err, "failed to parse llb proto op")
			}
			dgst := digest.FromBytes(dt)
			opMetadata, ok := def.Metadata[dgst]
			if !ok {
				opMetadata = pb.OpMetadata{}
			}
			c := llb.Constraints{Metadata: opMetadata}
			llb.IgnoreCache(&c)
			def.Metadata[dgst] = c.Metadata
		}
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

	solveOpt := client.SolveOpt{
		Exporter: "image",
		// ExporterAttrs is set later
		// LocalDirs is set later
		Frontend: "dockerfile.v0",
		// FrontendAttrs is set later
		ExportCache: clicontext.String("export-cache"),
		ImportCache: clicontext.StringSlice("import-cache"),
		Session:     []session.Attachable{authprovider.NewDockerAuthProvider()},
	}
	solveOpt.ExporterAttrs, err = attrMap(fmt.Sprintf("name=%s", clicontext.String("name")))
	if err != nil {
		return errors.Wrap(err, "invalid exporter-opt")
	}
	solveOpt.ExporterOutput, solveOpt.ExporterOutputDir, err = resolveExporterOutput(solveOpt.Exporter, solveOpt.ExporterAttrs["output"])
	if err != nil {
		return errors.Wrap(err, "invalid exporter-opt: output")
	}
	if solveOpt.ExporterOutput != nil || solveOpt.ExporterOutputDir != "" {
		delete(solveOpt.ExporterAttrs, "output")
	}

	exportCacheAttrs, err := attrMap(clicontext.StringSlice("export-cache-opt")...)
	if err != nil {
		return errors.Wrap(err, "invalid export-cache-opt")
	}
	if len(exportCacheAttrs) == 0 {
		exportCacheAttrs = map[string]string{"mode": "min"}
	}
	solveOpt.ExportCacheAttrs = exportCacheAttrs

	solveOpt.LocalDirs, err = attrMap("context=.", "dockerfile=.")
	if err != nil {
		return errors.Wrap(err, "invalid local")
	}

	var def *llb.Definition
	if clicontext.String("frontend") == "" {
		def, err = read(os.Stdin, clicontext)
		if err != nil {
			return err
		}
	} else {
		if clicontext.Bool("no-cache") {
			return errors.New("no-cache is not supported for frontends")
		}
	}

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
		return progressui.DisplaySolveStatus(context.TODO(), c, os.Stdout, displayCh)
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
	return c.App.Metadata["context"].(context.Context)
}

func resolveClient(c *cli.Context) (*client.Client, error) {
	opts := []client.ClientOpt{client.WithBlock()}
	ctx := commandContext(c)
	if span := opentracing.SpanFromContext(ctx); span != nil {
		opts = append(opts, client.WithTracer(span.Tracer()))
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return client.New(ctx, appdefaults.Address, opts...)
}
