package config

import (
	"fmt"
	"github.com/ykhdr/crack-hash/common/config"
	"github.com/ykhdr/crack-hash/common/consul"
	"github.com/ykhdr/crack-hash/worker/net"
)

type ServerConfig struct {
	Port    int    `kdl:"server-port"`
	Address string `kdl:"server-address"`
}

func (w *ServerConfig) Url() string {
	return fmt.Sprintf("%s:%d", w.Address, w.Port)
}

type WorkerConfig struct {
	ServerConfig
	ManagerUrl   string         `kdl:"manager-url"`
	ConsulConfig *consul.Config `kdl:"consul"`
}

func DefaultConfig() *WorkerConfig {
	return &WorkerConfig{
		ManagerUrl: "manager:8080",
		ConsulConfig: &consul.Config{
			Address: "consul:8500",
			Health: &consul.HealthConfig{
				Interval: "2s",
				Timeout:  "1s",
				Http:     "/api/health",
			},
		},
		ServerConfig: ServerConfig{
			Port:    8080,
			Address: "0.0.0.0",
		},
	}
}

func InitializeConfig(args []string) (*WorkerConfig, error) {
	cfg, err := config.InitializeConfig[WorkerConfig](args, *DefaultConfig())
	if err != nil {
		return nil, fmt.Errorf("error initialize config: %w", err)
	}
	addr, err := net.FindAvailableIPv4Addr()
	if err == nil {
		cfg.ServerConfig.Address = string(addr)
	} else {
		return nil, fmt.Errorf("error find available address: %w", err)
	}
	//todo выделить отдельное поле для http
	cfg.ConsulConfig.Health.Http = fmt.Sprintf("http://%s%s", cfg.Url(), cfg.ConsulConfig.Health.Http)
	return cfg, nil
}
