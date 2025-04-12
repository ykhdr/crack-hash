package config

import (
	"github.com/ykhdr/crack-hash/common/amqp"
	"github.com/ykhdr/crack-hash/common/config"
	"github.com/ykhdr/crack-hash/common/consul"
	"github.com/ykhdr/crack-hash/common/store/mongo"
	"time"
)

type DispatcherConfig struct {
	RequestQueueSize int           `kdl:"request-queue-size"`
	DispatchTimeout  time.Duration `kdl:"dispatch-timeout"`
	RequestTimeout   time.Duration `kdl:"request-timeout"`
	ReconnectTimeout time.Duration `kdl:"reconnect-timeout"`
	HealthTimeout    time.Duration `kdl:"health-timeout"`
}

type ManagerConfig struct {
	config.LogConfig
	ApiServerAddr    string            `kdl:"api-server-addr"`
	WorkerServerAddr string            `kdl:"worker-server-addr"`
	DispatcherConfig *DispatcherConfig `kdl:"dispatcher"`
	AmqpConfig       *amqp.Config      `kdl:"amqp"`
	ConsulConfig     *consul.Config    `kdl:"consul"`
	MongoDBConfig    *mongo.Config     `kdl:"mongo"`
}

func DefaultConfig() *ManagerConfig {
	return &ManagerConfig{
		ApiServerAddr:    "127.0.0.1:8080",
		WorkerServerAddr: "127.0.0.1:8081",
		DispatcherConfig: &DispatcherConfig{
			RequestQueueSize: 1024,
			DispatchTimeout:  5 * time.Second,
			RequestTimeout:   30 * time.Second,
			ReconnectTimeout: 1 * time.Second,
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
		LogConfig: config.LogConfig{
			LogLevel: "info",
		},
		AmqpConfig: &amqp.Config{
			URI:              "amqp://guest:guest@localhost:5672/",
			Username:         "guest",
			Password:         "guest",
			ReconnectTimeout: 1 * time.Second,
			ConsumerConfig: &amqp.ConsumerConfig{
				Queue: "crack-response-queue",
			},
			PublisherConfig: &amqp.PublisherConfig{
				Exchange:   "crack-request-exchange",
				RoutingKey: "crack-request",
			},
		},
		MongoDBConfig: &mongo.Config{
			ClientConfig: mongo.ClientConfig{
				URI:      "mongodb://admin:secret@mongo-primary:27017,mongo-secondary1:27017,mongo-secondary2:27017/?replicaSet=rs0&authSource=admin",
				Username: "admin",
				Password: "secret",
			},
			Database: "requests",
		},
	}
}

func InitializeConfig(args []string) (*ManagerConfig, error) {
	return config.InitializeConfig[ManagerConfig](args, *DefaultConfig())
}
