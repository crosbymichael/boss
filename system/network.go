package system

import (
	"fmt"
	"net"

	"github.com/containerd/containerd"
	networking "github.com/containerd/go-cni"
)

type cni struct {
	network networking.CNI
}

func (n *cni) Create(task containerd.Task) (string, error) {
	result, err := n.network.Setup(task.ID(), fmt.Sprintf("/proc/%d/ns/net", task.Pid()))
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
	return ip.String(), nil
}

func (n *cni) Remove(_ containerd.Container) error {
	return nil
}

type host struct {
}

func (n *host) Create(_ containerd.Task) (string, error) {
	// TODO: get default route's ip
	return "", nil
}

func (n *host) Remove(_ containerd.Container) error {
	return nil
}

type none struct {
}

func (n *none) Create(_ containerd.Task) (string, error) {
	return "", nil
}

func (n *none) Remove(_ containerd.Container) error {
	return nil
}
