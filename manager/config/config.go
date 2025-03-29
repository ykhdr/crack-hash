package config

import (
	"github.com/ykhdr/crack-hash/common/config"
	"github.com/ykhdr/crack-hash/common/consul"
	"time"
)

type DispatcherConfig struct {
	RequestQueueSize int           `kdl:"request-queue-size"`
	DispatchTimeout  time.Duration `kdl:"dispatch-timeout"`
	RequestTimeout   time.Duration `kdl:"request-timeout"`
	HealthTimeout    time.Duration `kdl:"health-timeout"`
}

type ManagerConfig struct {
	config.LogConfig
	ApiServerAddr    string            `kdl:"api-server-addr"`
	WorkerServerAddr string            `kdl:"worker-server-addr"`
	DispatcherConfig *DispatcherConfig `kdl:"dispatcher"`
	ConsulConfig     *consul.Config    `kdl:"consul"`
}

func DefaultConfig() *ManagerConfig {
	return &ManagerConfig{
		ApiServerAddr:    "127.0.0.1:8080",
		WorkerServerAddr: "127.0.0.1:8081",
		DispatcherConfig: &DispatcherConfig{
			RequestQueueSize: 1024,
			DispatchTimeout:  5 * time.Second,
			RequestTimeout:   30 * time.Second,
			HealthTimeout:    5 * time.Second,
		},
		ConsulConfig: &consul.Config{
			Address: "consul:8500",
			Health: &consul.HealthConfig{
				Interval: "5s",
				Timeout:  "30s",
				Http:     "/api/health",
			},
		},
	}
}

func InitializeConfig(args []string) (*ManagerConfig, error) {
	return config.InitializeConfig[ManagerConfig](args, *DefaultConfig())
}
