package main

type Config struct {
	ID        string     `toml:"id"`
	Image     string     `toml:"image"`
	Resources *Resources `toml:"resources"`
	Network   Network    `toml:"network"`
}

type Resources struct {
	CPU    float64 `toml:"cpu"`
	Memory int     `toml:"memory"`
}

type Network struct {
	Host bool `toml:"host"`
}
