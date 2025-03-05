package consul

import "github.com/hashicorp/consul/api"

type HealthConfig struct {
	Interval          string `kdl:"interval"`
	Timeout           string `kdl:"timeout"`
	Http              string `kdl:"http"`
	DeregisterTimeout string `kdl:"deregister-timeout"`
}

func (c *HealthConfig) toApiConfig() *api.AgentServiceCheck {
	return &api.AgentServiceCheck{
		HTTP:                           c.Http,
		Timeout:                        c.Timeout,
		Interval:                       c.Interval,
		DeregisterCriticalServiceAfter: c.DeregisterTimeout,
	}
}

type Config struct {
	Address string        `kdl:"address"`
	Health  *HealthConfig `kdl:"health"`
}

func (c *Config) toApiConfig() *api.Config {
	cfg := api.DefaultConfig()
	cfg.Address = c.Address
	return cfg
}
