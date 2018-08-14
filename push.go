package main

import (
	"context"
	"os"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/containerd/cmd/ctr/commands/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/pkg/progress"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
)

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
