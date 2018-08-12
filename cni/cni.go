package cni

import (
	"context"
	"io/ioutil"
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
	path := filepath.Join(config.Net, task.ID())
	if _, err := os.Lstat(filepath.Join(path, "ip")); err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		if err := os.MkdirAll(path, 0700); err != nil {
			return "", err
		}
		nspath := filepath.Join(path, "ns")
		if err := createNetns(nspath); err != nil {
			return "", err
		}
		result, err := n.network.Setup(task.ID(), nspath)
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
		if err := ioutil.WriteFile(filepath.Join(path, "ip"), []byte(ip.String()), 0666); err != nil {
			return "", err
		}
		if n.nt == "macvlan" {
			route.Remove(ip.String())
			if err := route.Add(ip.String()); err != nil {
				return "", err
			}
		}
	}
	data, err := ioutil.ReadFile(filepath.Join(path, "ip"))
	return string(data), err
}

func (n *cni) Remove(ctx context.Context, c containerd.Container) error {
	var (
		path   = filepath.Join(config.Net, c.ID())
		nspath = filepath.Join(path, "ns")
	)
	if err := n.network.Remove(c.ID(), nspath); err != nil {
		return err
	}
	if err := unix.Unmount(nspath, 0); err != nil {
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
	return os.RemoveAll(path)
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
