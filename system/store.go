package system

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/oci"
	"github.com/crosbymichael/boss/config"
	"github.com/hashicorp/consul/api"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

var ErrConfigStoreNotSupported = errors.New("config store not enabled, you need consul")

type nullStore struct {
}

func (l *nullStore) Write(_ context.Context, c *config.Container) error {
	if len(c.Configs) > 0 {
		return ErrConfigStoreNotSupported
	}
	return nil
}

func (l *nullStore) Watch(_ context.Context, _ containerd.Container, _ *config.Container) (<-chan error, error) {
	return make(chan error), nil
}

type configStore struct {
	consul *api.Client
}

func (l *configStore) Write(ctx context.Context, c *config.Container) error {
	kv := l.consul.KV()
	for _, f := range c.Configs {
		if f.Content == "" {
			continue
		}
		p, _, err := kv.Get(f.Source, nil)
		// don't overwrite configs
		if err != nil || p == nil || p.Value == nil {
			if _, err := kv.Put(&api.KVPair{
				Key:   f.Source,
				Value: []byte(f.Content),
			}, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

func (l *configStore) Watch(ctx context.Context, c containerd.Container, cfg *config.Container) (<-chan error, error) {
	var (
		ch = make(chan error, len(cfg.Configs))
		kv = l.consul.KV()
	)
	spec, err := c.Spec(ctx)
	if err != nil {
		return nil, err
	}
	var templates []*Template
	for name, f := range cfg.Configs {
		data, meta, err := kv.Get(f.Source, nil)
		if err != nil {
			return nil, err
		}
		if data == nil || data.Value == nil {
			continue
		}
		templates = append(templates, &Template{
			Index:     meta.LastIndex,
			Name:      name,
			File:      f,
			Data:      data.Value,
			Container: c,
			Spec:      spec,
		})
	}
	for _, t := range templates {
		if err := t.Render(ctx); err != nil {
			return nil, err
		}
		go t.Watch(ctx, kv, ch)
	}
	return ch, nil
}

type Template struct {
	Index     uint64
	Container containerd.Container
	Name      string
	File      config.File
	Data      []byte
	Spec      *oci.Spec
}

func (t *Template) Render(ctx context.Context) error {
	path := filepath.Join(config.State, t.Container.ID(), "configs", t.Name)
	if err := os.MkdirAll(filepath.Dir(path), 0711); err != nil {
		return err
	}
	f, err := ioutil.TempFile("/run/boss", t.Name)
	if err != nil {
		return err
	}
	if err := f.Chown(int(t.Spec.Process.User.UID), int(t.Spec.Process.User.GID)); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(t.Data); err != nil {
		f.Close()
		return err
	}
	f.Close()
	return os.Rename(f.Name(), path)
}

func (t *Template) Watch(ctx context.Context, kv *api.KV, ch chan error) {
	for {
		select {
		case <-ctx.Done():
			ch <- ctx.Err()
			return
		default:
			data, meta, err := kv.Get(t.File.Source, &api.QueryOptions{WaitIndex: t.Index})
			if err != nil {
				ch <- err
				time.Sleep(2 * time.Second)
			}
			t.Index = meta.LastIndex
			t.Data = data.Value
			if err := t.Render(ctx); err != nil {
				ch <- err
				continue
			}
			if t.File.Signal != "" {
				var sig syscall.Signal
				// TODO signal map for all sigs
				switch t.File.Signal {
				case "SIGHUP":
					sig = unix.SIGHUP
				case "SIGKILL":
					sig = unix.SIGKILL
				case "SIGTERM":
					sig = unix.SIGTERM
				case "SIGUSR1":
					sig = unix.SIGUSR1
				case "SIGUSR2":
					sig = unix.SIGUSR2
				default:
					ch <- errors.Errorf("unsupported signal %q", t.File.Signal)
					continue
				}
				task, err := t.Container.Task(ctx, nil)
				if err != nil {
					ch <- err
					continue
				}
				if err := task.Kill(ctx, sig); err != nil {
					ch <- err
				}
			}
		}
	}
}
