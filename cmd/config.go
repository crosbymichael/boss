package cmd

import v1 "github.com/crosbymichael/boss/api/v1"

const Version = "v1"

type Container struct {
	ConfigVersion string             `toml:"config_version"`
	ID            string             `toml:"id"`
	Image         string             `toml:"image"`
	Resources     *Resources         `toml:"resources"`
	GPUs          *GPUs              `toml:"gpus"`
	Mounts        []Mount            `toml:"mounts"`
	Env           []string           `toml:"env"`
	Args          []string           `toml:"args"`
	UID           *int               `toml:"uid"`
	GID           *int               `toml:"gid"`
	Network       string             `toml:"network"`
	Services      map[string]Service `toml:"services"`
	Configs       map[string]File    `toml:"configs"`
	Readonly      bool               `toml:"readonly"`
	Capabilities  []string           `toml:"caps"`
	Volumes       map[string]Volume  `toml:"volumes"`
	Privileged    bool               `toml:"privileged"`
}

func (c *Container) Proto() *v1.Container {
	container := &v1.Container{
		ID:      c.ID,
		Image:   c.Image,
		Network: c.Network,
		Process: &v1.Process{
			Args:         c.Args,
			Env:          c.Env,
			Capabilities: c.Capabilities,
		},
		Readonly:   c.Readonly,
		Privileged: c.Privileged,
		Services:   make(map[string]*v1.Service),
		Configs:    make(map[string]*v1.Config),
	}
	for _, m := range c.Mounts {
		container.Mounts = append(container.Mounts, &v1.Mount{
			Type:        m.Type,
			Source:      m.Source,
			Destination: m.Destination,
			Options:     m.Options,
		})
	}
	if c.Resources != nil {
		container.Resources = &v1.Resources{
			Cpus:   c.Resources.CPU,
			Memory: c.Resources.Memory,
			Score:  c.Resources.Score,
			NoFile: c.Resources.NoFile,
		}
	}
	if c.GPUs != nil {
		container.Gpus = &v1.GPUs{
			Devices:      c.GPUs.Devices,
			Capabilities: c.GPUs.Capbilities,
		}
	}
	if c.UID != nil {
		gid := 0
		if c.GID != nil {
			gid = *c.GID
		}
		container.Process.User = &v1.User{
			Uid: uint32(*c.UID),
			Gid: uint32(gid),
		}
	}
	for name, s := range c.Services {
		container.Services[name] = &v1.Service{
			Port:   s.Port,
			Labels: s.Labels,
			Url:    s.URL,
		}
		if s.CheckType != "" {
			container.Services[name].Check = &v1.HealthCheck{
				Type:     string(s.CheckType),
				Interval: s.CheckInterval,
				Timeout:  s.CheckTimeout,
				Method:   s.CheckMethod,
			}
		}
	}
	for name, cfg := range c.Configs {
		container.Configs[name] = &v1.Config{
			Path:    cfg.Path,
			Source:  cfg.Source,
			Signal:  cfg.Signal,
			Content: cfg.Content,
		}
	}
	for id, vol := range c.Volumes {
		container.Volumes = append(container.Volumes, &v1.Volume{
			ID:          id,
			Destination: vol.Destination,
			Rw:          vol.RW,
		})
	}
	return container
}

type File struct {
	Path    string `toml:"path"`
	Source  string `toml:"source"`
	Content string `toml:"content"`
	// Signal to be sent when the config changes
	Signal string `toml:"signal"`
}

type Service struct {
	Port          int64     `toml:"port"`
	Labels        []string  `toml:"labels"`
	URL           string    `toml:"url"`
	CheckType     CheckType `toml:"check_type"`
	CheckInterval int64     `toml:"check_interval"`
	CheckTimeout  int64     `toml:"check_timeout"`
	CheckMethod   string    `toml:"check_method"`
}

type CheckType string

const (
	HTTP CheckType = "http"
	TCP  CheckType = "tcp"
	GRPC CheckType = "grpc"
)

type Resources struct {
	CPU    float64 `toml:"cpu"`
	Memory int64   `toml:"memory"`
	Score  int64   `toml:"score"`
	NoFile uint64  `toml:"no_file"`
}

type GPUs struct {
	Devices     []int64  `toml:"devices"`
	Capbilities []string `toml:"capabilities"`
}

type Mount struct {
	Type        string   `toml:"type"`
	Source      string   `toml:"source"`
	Destination string   `toml:"destination"`
	Options     []string `toml:"options"`
}

type Volume struct {
	Destination string `toml:"destination"`
	RW          bool   `toml:"rw"`
}
