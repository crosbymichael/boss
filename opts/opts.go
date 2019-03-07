package opts

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd"
	api "github.com/containerd/containerd/api/types"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/contrib/apparmor"
	"github.com/containerd/containerd/contrib/nvidia"
	"github.com/containerd/containerd/contrib/seccomp"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/typeurl"
	v1 "github.com/crosbymichael/boss/api/v1"
	"github.com/gogo/protobuf/types"
	is "github.com/opencontainers/image-spec/specs-go/v1"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const (
	CurrentConfig          = "io.boss/container"
	LastConfig             = "io.boss/container.last"
	IPLabel                = "io/boss/container.ip"
	RestoreCheckpointLabel = "io/boss/restore.checkpoint"
)

// WithBossConfig is a containerd.NewContainerOpts for spec and container configuration
func WithBossConfig(volumeRoot string, config *v1.Container, image containerd.Image) func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
	return func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
		// generate the spec
		if err := containerd.WithNewSpec(specOpt(volumeRoot, config, image))(ctx, client, c); err != nil {
			return err
		}
		// save the config as a container extension
		return containerd.WithContainerExtension(CurrentConfig, config)(ctx, client, c)
	}
}

func WithSetPreviousConfig(ctx context.Context, client *containerd.Client, c *containers.Container) error {
	c.Extensions[LastConfig] = c.Extensions[CurrentConfig]
	return nil
}

func WithRollback(ctx context.Context, client *containerd.Client, c *containers.Container) error {
	d := c.Extensions[LastConfig]
	if d.Value == nil {
		return nil
	}
	c.Extensions[CurrentConfig] = d
	return nil
}

func specOpt(volumeRoot string, config *v1.Container, image containerd.Image) oci.SpecOpts {
	opts := []oci.SpecOpts{
		oci.WithImageConfigArgs(image, config.Process.Args),
		oci.WithHostLocaltime,
		oci.WithNoNewPrivileges,
		apparmor.WithDefaultProfile("boss"),
		seccomp.WithDefaultProfile(),
		oci.WithEnv(config.Process.Env),
		withMounts(config.Mounts),
		withVolumes(volumeRoot, config.Volumes),
		withConfigs(config.Configs),
	}
	if config.Privileged {
		opts = append(opts, oci.WithPrivileged)
	}
	if config.Network == "host" {
		opts = append(opts, oci.WithHostHostsFile, oci.WithHostResolvconf, oci.WithHostNamespace(specs.NetworkNamespace))
	} else if config.Network == "cni" {
		opts = append(opts, withBossResolvconf, withContainerHostsFile, oci.WithLinuxNamespace(specs.LinuxNamespace{
			Type: specs.NetworkNamespace,
			Path: v1.NetworkPath(config.ID),
		}),
			oci.WithHostname(config.ID),
		)
	}
	if config.Resources != nil {
		opts = append(opts, withResources(config.Resources))
	}
	if config.Gpus != nil {
		opts = append(opts, nvidia.WithGPUs(
			nvidia.WithDevices(ints(config.Gpus.Devices)...),
			nvidia.WithCapabilities(toGpuCaps(config.Gpus.Capabilities)...),
		),
		)
	}
	if config.Process.User != nil {
		opts = append(opts, oci.WithUIDGID(config.Process.User.Uid, config.Process.User.Gid))
	}
	if config.Readonly {
		opts = append(opts, oci.WithRootFSReadonly())
	}
	// make sure this opt is run after the user has been set
	opts = append(opts, withProcessCaps(config.Process.Capabilities))
	return oci.Compose(opts...)
}

func withProcessCaps(capabilities []string) oci.SpecOpts {
	return func(ctx context.Context, _ oci.Client, c *containers.Container, s *oci.Spec) error {
		set := make(map[string]struct{})
		for _, s := range s.Process.Capabilities.Bounding {
			set[s] = struct{}{}
		}
		for _, cc := range capabilities {
			set[cc] = struct{}{}
		}
		ss := stringSet(set)
		s.Process.Capabilities.Bounding = ss
		s.Process.Capabilities.Effective = ss
		s.Process.Capabilities.Permitted = ss
		s.Process.Capabilities.Inheritable = ss
		if s.Process.User.UID != 0 {
			s.Process.Capabilities.Ambient = ss
		}
		return nil
	}
}

func stringSet(set map[string]struct{}) (o []string) {
	for k := range set {
		o = append(o, k)
	}
	return o
}

func ints(i []int64) (o []int) {
	for _, ii := range i {
		o = append(o, int(ii))
	}
	return o
}

func toStrings(ss []string) map[string]string {
	m := make(map[string]string, len(ss))
	for _, s := range ss {
		parts := strings.SplitN(s, "=", 2)
		m[parts[0]] = parts[1]
	}
	return m
}

func toGpuCaps(ss []string) (o []nvidia.Capability) {
	for _, s := range ss {
		o = append(o, nvidia.Capability(s))
	}
	return o
}

func withResources(r *v1.Resources) oci.SpecOpts {
	return func(ctx context.Context, _ oci.Client, c *containers.Container, s *oci.Spec) error {
		if r.Memory > 0 {
			limit := r.Memory * 1024 * 1024
			s.Linux.Resources.Memory = &specs.LinuxMemory{
				Limit: &limit,
			}
		}
		if r.Cpus > 0 {
			period := uint64(100000)
			quota := int64(r.Cpus * 100000.0)
			s.Linux.Resources.CPU = &specs.LinuxCPU{
				Quota:  &quota,
				Period: &period,
			}
		}
		if r.Score != 0 {
			score := int(r.Score)
			s.Process.OOMScoreAdj = &score
		}
		if r.NoFile > 0 {
			s.Process.Rlimits = []specs.POSIXRlimit{
				{
					Type: "RLIMIT_NOFILE",
					Hard: r.NoFile,
					Soft: r.NoFile,
				},
			}
		}
		return nil
	}
}

func withMounts(mounts []*v1.Mount) oci.SpecOpts {
	return func(ctx context.Context, _ oci.Client, c *containers.Container, s *oci.Spec) error {
		for _, cm := range mounts {
			if cm.Type == "bind" {
				// create source if it does not exist
				if err := createHostDir(cm.Source, int(s.Process.User.UID), int(s.Process.User.GID)); err != nil {
					return err
				}
			}
			s.Mounts = append(s.Mounts, specs.Mount{
				Type:        cm.Type,
				Source:      cm.Source,
				Destination: cm.Destination,
				Options:     cm.Options,
			})
		}
		return nil
	}
}

func createHostDir(path string, uid, gid int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.Mkdir(path, 0755); err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	if err := os.Chown(path, uid, gid); err != nil {
		return err
	}
	return nil
}

func withVolumes(root string, volumes []*v1.Volume) oci.SpecOpts {
	return func(ctx context.Context, _ oci.Client, c *containers.Container, s *oci.Spec) error {
		for _, cm := range volumes {
			if root == "" {
				return errors.New("no volume_root specified")
			}
			source := filepath.Join(root, cm.ID)
			if err := createHostDir(source, int(s.Process.User.UID), int(s.Process.User.GID)); err != nil {
				return err
			}
			opts := []string{"bind"}
			if cm.Rw {
				opts = append(opts, "rw")
			} else {
				opts = append(opts, "ro")
			}
			s.Mounts = append(s.Mounts, specs.Mount{
				Type:        "bind",
				Source:      source,
				Destination: cm.Destination,
				Options:     opts,
			})
		}
		return nil
	}
}

func withConfigs(files map[string]*v1.Config) oci.SpecOpts {
	return func(ctx context.Context, _ oci.Client, c *containers.Container, s *oci.Spec) error {
		for name, f := range files {
			s.Mounts = append(s.Mounts, specs.Mount{
				Type:        "bind",
				Source:      v1.ConfigPath(c.ID, name),
				Destination: f.Path,
				Options: []string{
					"ro", "rbind",
				},
			})
		}
		return nil
	}
}

func withContainerHostsFile(ctx context.Context, _ oci.Client, c *containers.Container, s *oci.Spec) error {
	id := c.ID
	if err := os.MkdirAll(filepath.Join(v1.Root, id), 0711); err != nil {
		return err
	}
	hostname := s.Hostname
	if hostname == "" {
		hostname = id
	}
	path := filepath.Join(v1.Root, id, "hosts")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := f.Chmod(0666); err != nil {
		return err
	}
	if _, err := f.WriteString("127.0.0.1       localhost\n"); err != nil {
		return err
	}
	if _, err := f.WriteString(fmt.Sprintf("127.0.0.1       %s\n", hostname)); err != nil {
		return err
	}
	if _, err := f.WriteString("::1     localhost ip6-localhost ip6-loopback\n"); err != nil {
		return err
	}
	s.Mounts = append(s.Mounts, specs.Mount{
		Destination: "/etc/hosts",
		Type:        "bind",
		Source:      path,
		Options:     []string{"rbind", "ro"},
	})
	return nil
}

func withBossResolvconf(ctx context.Context, _ oci.Client, c *containers.Container, s *oci.Spec) error {
	s.Mounts = append(s.Mounts, specs.Mount{
		Destination: "/etc/resolv.conf",
		Type:        "bind",
		Source:      filepath.Join(v1.Root, c.ID, "resolv.conf"),
		Options:     []string{"rbind", "ro"},
	})
	return nil
}

func GetConfig(ctx context.Context, container containerd.Container) (*v1.Container, error) {
	info, err := container.Info(ctx)
	if err != nil {
		return nil, err
	}
	return GetConfigFromInfo(ctx, info)
}

func GetConfigFromInfo(ctx context.Context, info containers.Container) (*v1.Container, error) {
	d := info.Extensions[CurrentConfig]
	return UnmarshalConfig(&d)
}

var ErrOldConfigFormat = errors.New("old config format on container")

func UnmarshalConfig(any *types.Any) (*v1.Container, error) {
	v, err := typeurl.UnmarshalAny(any)
	if err != nil {
		return nil, err
	}
	c, ok := v.(*v1.Container)
	if !ok {
		return nil, ErrOldConfigFormat
	}
	return c, nil
}

// WithIP sets the ip on the container
func WithIP(ip string) containerd.UpdateContainerOpts {
	return func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
		if c.Labels == nil {
			c.Labels = make(map[string]string)
		}
		c.Labels[IPLabel] = ip
		return nil
	}
}

func WithRestore(m *is.Descriptor) containerd.NewContainerOpts {
	return func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
		if c.Extensions == nil {
			c.Extensions = make(map[string]types.Any)
		}
		v := &api.Descriptor{
			MediaType: m.MediaType,
			Size_:     m.Size,
			Digest:    m.Digest,
		}
		any, err := typeurl.MarshalAny(v)
		if err != nil {
			return err
		}
		c.Extensions[RestoreCheckpointLabel] = *any
		return nil
	}
}

func WithoutRestore(ctx context.Context, client *containerd.Client, c *containers.Container) error {
	if c.Extensions == nil {
		c.Extensions = make(map[string]types.Any)
	}
	delete(c.Extensions, RestoreCheckpointLabel)
	return nil
}

func WithTaskRestore(desc *api.Descriptor) containerd.NewTaskOpts {
	return func(ctx context.Context, client *containerd.Client, ti *containerd.TaskInfo) error {
		ti.Checkpoint = desc
		return nil
	}
}

func GetRestoreDesc(ctx context.Context, c containerd.Container) (*api.Descriptor, error) {
	ex, err := c.Extensions(ctx)
	if err != nil {
		return nil, err
	}
	any, ok := ex[RestoreCheckpointLabel]
	if !ok {
		return nil, nil
	}
	v, err := typeurl.UnmarshalAny(&any)
	if err != nil {
		return nil, err
	}
	return v.(*api.Descriptor), nil
}
