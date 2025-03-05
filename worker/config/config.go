package config

import (
	"errors"
	"fmt"
	"github.com/ykhdr/crack-hash/common/config"
	"github.com/ykhdr/crack-hash/common/consul"
	"net"
)

type WorkerConfig struct {
	ServerPort   int            `kdl:"server-port"`
	ManagerUrl   string         `kdl:"manager-url"`
	ConsulConfig *consul.Config `kdl:"consul"`
	Address      string
	Url          string
}

func DefaultConfig() *WorkerConfig {
	return &WorkerConfig{
		ServerPort: 8080,
		ManagerUrl: "manager:8080",
		ConsulConfig: &consul.Config{
			Address: "consul:8500",
			Health: &consul.HealthConfig{
				Interval: "2s",
				Timeout:  "1s",
				Http:     "/api/health",
			},
		},
	}
}

func InitializeConfig(args []string) (res *WorkerConfig, _ error) {
	cfg := *DefaultConfig()
	addr, err := findAvailableIPv4Addr()
	if err != nil {
		return nil, err
	}
	defer func() {
		res.Address = addr
		res.Url = fmt.Sprintf("%s:%d", addr, res.ServerPort)
		res.ConsulConfig.Health.Http = "http://" + res.Url + res.ConsulConfig.Health.Http
	}()
	return config.InitializeConfig[WorkerConfig](args, cfg)
}

func findAvailableIPv4Addr() (string, error) {
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
					return ip4.String(), nil
				}
			}
		}
	}
	return "", errors.New("no valid network interface found")
}
