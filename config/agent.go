package config

const agentUnit = `[Unit]
Description=boss agent
After=containerd.service network.target

[Service]
ExecStartPre=/bin/mount -a
ExecStart=/usr/local/bin/boss agent
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target`

type Agent struct {
	PlainRemotes []string `toml:"plain_remotes"`
	VolumeRoot   string   `toml:"volume_root"`
}

func (s *Agent) Name() string {
	return "agent"
}
