package cni

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/containerd/containerd"
	networking "github.com/containerd/go-cni"
	"github.com/crosbymichael/boss/config"
	"github.com/crosbymichael/boss/route"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func New(t, iface string, n networking.CNI) (config.Network, error) {
	if t == "macvlan" {
		if err := route.Create(iface); err != nil {
			return nil, err
		}
	}
	return &cni{
		network: n,
		nt:      t,
	}, nil
}

type cni struct {
	network networking.CNI
	nt      string
}

func (n *cni) Create(ctx context.Context, task containerd.Container) (string, error) {
	path := config.NetworkPath(task.ID())
	if _, err := os.Lstat(path); err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return "", err
		}
		if err := createNetns(path); err != nil {
			return "", err
		}
		result, err := n.network.Setup(task.ID(), path)
		if err != nil {
			return "", err
		}
		var ip net.IP
		for _, ipc := range result.Interfaces["eth0"].IPConfigs {
			if f := ipc.IP.To4(); f != nil {
				ip = f
				break
			}
		}
		if err := task.Update(ctx, config.WithIP(ip.String())); err != nil {
			return "", err
		}
		if n.nt == "macvlan" {
			route.Remove(ip.String())
			if err := route.Add(ip.String()); err != nil {
				return "", err
			}
		}
		return ip.String(), nil
	}
	l, err := task.Labels(ctx)
	if err != nil {
		return "", err
	}
	return l[config.IPLabel], nil
}

func (n *cni) Remove(ctx context.Context, c containerd.Container) error {
	path := config.NetworkPath(c.ID())
	if err := n.network.Remove(c.ID(), path); err != nil {
		return err
	}
	if err := unix.Unmount(path, 0); err != nil {
		return err
	}
	if n.nt == "macvlan" {
		info, err := c.Info(ctx)
		if err != nil {
			return err
		}
		ip := info.Labels[config.IPLabel]
		if ip != "" {
			if err := route.Remove(ip); err != nil {
				return err
			}
		}
	}
	// FIXME this could cause issues later but whatever...
	return os.RemoveAll(filepath.Dir(path))
}

func createNetns(path string) error {
	cmd := exec.Command("boss", "network", "create", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: unix.CLONE_NEWNET,
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}
