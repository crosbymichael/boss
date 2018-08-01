// stole from buildkit, tonis said it was ok
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/containerd/console"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/containerd/cmd/ctr/commands/content"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/pkg/progress"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/util/appdefaults"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/opencontainers/image-spec/specs-go/v1"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
)

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
	},
	Action: func(clix *cli.Context) error {
		if err := build(clix); err != nil {
			return err
		}
		if !clix.Bool("push") {
			return nil
		}
		ref := clix.String("name")
		ctx := namespaces.WithNamespace(context.Background(), clix.GlobalString("namespace"))
		client, err := containerd.New(
			defaults.DefaultAddress,
			containerd.WithDefaultRuntime("io.containerd.runc.v1"),
		)
		if err != nil {
			return err
		}
		defer client.Close()
		return push(ctx, client, ref, clix)
	},
}

func push(ctx context.Context, client *containerd.Client, ref string, clix *cli.Context) error {
	var (
		local = ref
		desc  v1.Descriptor
	)
	if ref == "" {
		return errors.New("please provide a remote image reference to push")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	img, err := client.ImageService().Get(ctx, local)
	if err != nil {
		return errors.Wrap(err, "unable to resolve image to manifest")
	}
	desc = img.Target

	resolver, err := commands.GetResolver(ctx, clix)
	if err != nil {
		return err
	}
	ongoing := newPushJobs(commands.PushTracker)

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		log.G(ctx).WithField("image", ref).WithField("digest", desc.Digest).Debug("pushing")

		jobHandler := images.HandlerFunc(func(ctx context.Context, desc v1.Descriptor) ([]v1.Descriptor, error) {
			ongoing.add(remotes.MakeRefKey(ctx, desc))
			return nil, nil
		})

		return client.Push(ctx, ref, desc,
			containerd.WithResolver(resolver),
			containerd.WithImageHandler(jobHandler),
		)
	})

	errs := make(chan error)
	go func() {
		defer close(errs)
		errs <- eg.Wait()
	}()

	var (
		ticker = time.NewTicker(100 * time.Millisecond)
		fw     = progress.NewWriter(os.Stdout)
		start  = time.Now()
		done   bool
	)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fw.Flush()

			tw := tabwriter.NewWriter(fw, 1, 8, 1, ' ', 0)

			content.Display(tw, ongoing.status(), start)
			tw.Flush()

			if done {
				fw.Flush()
				return nil
			}
		case err := <-errs:
			if err != nil {
				return err
			}
			done = true
		case <-ctx.Done():
			done = true // allow ui to update once more
		}
	}

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
	return client.New(ctx, appdefaults.Address, opts...)
}

type pushjobs struct {
	jobs    map[string]struct{}
	ordered []string
	tracker docker.StatusTracker
	mu      sync.Mutex
}

func newPushJobs(tracker docker.StatusTracker) *pushjobs {
	return &pushjobs{
		jobs:    make(map[string]struct{}),
		tracker: tracker,
	}
}

func (j *pushjobs) add(ref string) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if _, ok := j.jobs[ref]; ok {
		return
	}
	j.ordered = append(j.ordered, ref)
	j.jobs[ref] = struct{}{}
}

func (j *pushjobs) status() []content.StatusInfo {
	j.mu.Lock()
	defer j.mu.Unlock()

	statuses := make([]content.StatusInfo, 0, len(j.jobs))
	for _, name := range j.ordered {
		si := content.StatusInfo{
			Ref: name,
		}

		status, err := j.tracker.GetStatus(name)
		if err != nil {
			si.Status = "waiting"
		} else {
			si.Offset = status.Offset
			si.Total = status.Total
			si.StartedAt = status.StartedAt
			si.UpdatedAt = status.UpdatedAt
			if status.Offset >= status.Total {
				if status.UploadUUID == "" {
					si.Status = "done"
				} else {
					si.Status = "committing"
				}
			} else {
				si.Status = "uploading"
			}
		}
		statuses = append(statuses, si)
	}

	return statuses
}
