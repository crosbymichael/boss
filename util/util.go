package util

import (
	"errors"
	"net"
)

var ErrIPAddressNotFound = errors.New("box: ip address for interface not found")

func GetIP(name string) (string, error) {
	i, err := net.InterfaceByName(name)
	if err != nil {
		return "", err
	}
	return getIPf(i, ipv4)
}

func getIPf(i *net.Interface, ipfunc func(n *net.IPNet) string) (string, error) {
	addrs, err := i.Addrs()
	if err != nil {
		return "", err
	}
	for _, a := range addrs {
		n, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		s := ipfunc(n)
		if s == "" {
			continue
		}
		return s, nil
	}
	return "", ErrIPAddressNotFound
}

func ipv4(n *net.IPNet) string {
	if n.IP.To4() == nil {
		return ""
	}
	return n.IP.To4().String()
}
