package main

import (
	"github.com/box/box/config/v1"
)

type Container struct {
	ID        string             `toml:"id"`
	Image     string             `toml:"image"`
	Resources *Resources         `toml:"resources"`
	GPUs      *GPUs              `toml:"gpus"`
	Mounts    []Mount            `toml:"mounts"`
	Env       []string           `toml:"env"`
	Args      []string           `toml:"args"`
	UID       *int               `toml:"uid"`
	GID       *int               `toml:"gid"`
	Network   string             `toml:"network"`
	Services  map[string]Service `toml:"services"`
	Configs   map[string]File    `toml:"configs"`
}

func (c *Container) Proto() *v1.Container {
	panic("proto not done")
	return nil
}

type File struct {
	Path    string `toml:"path"`
	Source  string `toml:"source"`
	Content string `toml:"content"`
	// Signal to be sent when the config changes
	Signal string `toml:"signal"`
}

type Service struct {
	Port          int       `toml:"port"`
	Labels        []string  `toml:"labels"`
	URL           string    `toml:"url"`
	CheckType     CheckType `toml:"check_type"`
	CheckInterval int       `toml:"check_interval"`
	CheckTimeout  int       `toml:"check_timeout"`
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
	Score  int     `toml:"score"`
	NoFile uint64  `toml:"no_file"`
}

type GPUs struct {
	Devices     []int    `toml:"devices"`
	Capbilities []string `toml:"capabilities"`
}

type Mount struct {
	Type        string   `toml:"type"`
	Source      string   `toml:"source"`
	Destination string   `toml:"destination"`
	Options     []string `toml:"options"`
}
