package config

import (
	"fmt"
	"github.com/ykhdr/crack-hash/common/amqp"
	"github.com/ykhdr/crack-hash/common/config"
	"github.com/ykhdr/crack-hash/common/consul"
	"github.com/ykhdr/crack-hash/worker/internal/hashcrack/strategy"
	"github.com/ykhdr/crack-hash/worker/internal/net"
	"time"
)

type ServerConfig struct {
	Port    int    `kdl:"server-port"`
	Address string `kdl:"server-address"`
}

func (w *ServerConfig) Url() string {
	return fmt.Sprintf("%s:%d", w.Address, w.Port)
}

type WorkerConfig struct {
	config.LogConfig
	ServerConfig
	ManagerUrl   string         `kdl:"manager-url"`
	ConsulConfig *consul.Config `kdl:"consul"`
	Strategy     string         `kdl:"strategy"`
	AmqpConfig   *amqp.Config   `kdl:"amqp"`
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
		LogConfig: config.LogConfig{
			LogLevel: "info",
		},
		Strategy: strategy.DefaultStrategyStr(),
		AmqpConfig: &amqp.Config{
			URI:              "amqp://guest:guest@localhost:5672/",
			Username:         "guest",
			Password:         "guest",
			ReconnectTimeout: 3 * time.Second,
			ConsumerConfig: &amqp.ConsumerConfig{
				Queue: "queue.crack.request",
			},
			PublisherConfig: &amqp.PublisherConfig{
				Exchange:   "exchange.crack.response",
				RoutingKey: "crack.response",
			},
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
	cfg.ConsulConfig.Health.Http = fmt.Sprintf("http://%s%s", cfg.Url(), cfg.ConsulConfig.Health.Http)
	return cfg, nil
}
