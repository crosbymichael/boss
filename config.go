package main

type Config struct {
	ID          string             `toml:"id"`
	Image       string             `toml:"image"`
	Resources   *Resources         `toml:"resources"`
	GPUs        *GPUs              `toml:"gpus"`
	Mounts      []Mount            `toml:"mounts"`
	Env         []string           `toml:"env"`
	Args        []string           `toml:"args"`
	Labels      []string           `toml:"labels"`
	HostNetwork bool               `toml:"host_network"`
	Services    map[string]Service `toml:"services"`
}

type Service struct {
	Port   int      `toml:"port"`
	Labels []string `toml:"labels"`
}

type Resources struct {
	CPU    float64 `toml:"cpu"`
	Memory int64   `toml:"memory"`
	Score  int     `toml:"score"`
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
