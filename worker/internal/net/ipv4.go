package net

import (
	"errors"
	"net"
)

type IPv4Addr string

var ErrNoValidNetworkInterfaceFound = errors.New("no valid network interface found")

func FindAvailableIPv4Addr() (IPv4Addr, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok {
				if ip4 := ipNet.IP.To4(); ip4 != nil {
					return IPv4Addr(ip4.String()), nil
				}
			}
		}
	}
	return "", ErrNoValidNetworkInterfaceFound
}
