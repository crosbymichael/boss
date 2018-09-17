package v1

import (
	"context"
	"path/filepath"

	"github.com/containerd/containerd"
)

const (
	Root             = "/var/lib/boss"
	state            = "/run/boss"
	DefaultRuntime   = "io.containerd.runc.v1"
	DefaultNamespace = "boss"
	// configuration keys
	PlainRemotesKey = "io.boss.agent.plain-remotes"
	VolumeRootKey   = "io.boss.agent.volume-root"
)

func StatePath(id string) string {
	return filepath.Join(state, id)
}

// Register is an object that registers and manages service information in its backend
type Register interface {
	Register(id, name, ip string, s *Service) error
	Deregister(id, name string) error
	EnableMaintainance(id, name, msg string) error
	DisableMaintainance(id, name string) error
}

type Network interface {
	Create(context.Context, containerd.Container) (string, error)
	Remove(context.Context, containerd.Container) error
}

func NetworkPath(id string) string {
	return filepath.Join(StatePath(id), "net")
}

func ConfigPath(id, name string) string {
	return filepath.Join(StatePath(id), "configs", name)
}
