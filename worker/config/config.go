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

func InitializeConfig(args []string) (*WorkerConfig, error) {
	cfg := *DefaultConfig()
	addr, err := findAvailableIPv4Addr()
	if err != nil {
		return nil, err
	}
	cfg.Address = addr
	cfg.Url = fmt.Sprintf("%s:%d", addr, cfg.ServerPort)
	return config.InitializeConfig[WorkerConfig](args, cfg)
}

func findAvailableIPv4Addr() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range ifaces {
		// Пропускаем выключенные (нет флага Up) и loopback-интерфейсы
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}

		for _, addr := range addrs {
			// Разбираем адрес, проверяем, IPv4 ли это
			if ipNet, ok := addr.(*net.IPNet); ok {
				if ip4 := ipNet.IP.To4(); ip4 != nil {
					return ip4.String(), nil
				}
			}
		}
	}

	return "", errors.New("no valid network interface found")
}
