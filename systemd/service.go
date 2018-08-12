package systemd

import (
	"crypto/md5"
	"encoding/hex"
	"os"
)

const (
	// Root to install the service file
	Root = "/lib/systemd/system"
	// Version is an incrementing version of the service file used.
	// increment it if you need a new one to be used over the previous
	// and handle the code for making sure the previous one is able to stop containers
	Version = 1
)

const service = `
[Unit]
Description=Boss container proxy for %i
Wants=network-online.target containerd.service
After=network-online.target containerd.service network.target

[Service]
ExecStartPre=/usr/local/bin/boss systemd exec-start-pre %i
ExecStart=/usr/local/bin/boss systemd exec-start %i
ExecStartPost=/usr/local/bin/boss systemd exec-start-post %i
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`

func writeService(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	_, err = f.WriteString(service)
	f.Close()
	return err
}

func getHash(data []byte) string {
	h := md5.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
