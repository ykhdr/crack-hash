package consul

import (
	"github.com/google/uuid"
	"github.com/hashicorp/consul/api"
)

type Service struct {
	id        string
	address   string
	isHealthy bool
}

func (s *Service) Id() string {
	return s.id
}

func (s *Service) Address() string {
	return s.address
}

func (s *Service) IsHealthy() bool {
	return s.isHealthy
}

type Client interface {
	HealthServices(serviceName string) ([]*Service, error)
	CatalogServices(serviceName string) ([]*Service, error)
	RegisterService(serviceName, address string, port int) error
}

type client struct {
	cfg    *Config
	client *api.Client
}

func NewClient(cfg *Config) (Client, error) {
	cl, err := api.NewClient(cfg.toApiConfig())
	if err != nil {
		return nil, err
	}
	return &client{client: cl, cfg: cfg}, nil
}

func (c *client) HealthServices(serviceName string) ([]*Service, error) {
	entries, _, err := c.client.Health().Service(serviceName, "", true, nil)
	if err != nil {
		return nil, err
	}
	var services []*Service
	for _, srv := range entries {
		services = append(services, &Service{
			id:        srv.Service.ID,
			address:   srv.Service.Address,
			isHealthy: true,
		})
	}
	return services, nil
}

func (c *client) CatalogServices(serviceName string) ([]*Service, error) {
	entries, _, err := c.client.Catalog().Service(serviceName, "", nil)
	if err != nil {
		return nil, err
	}
	var services []*Service
	for _, srv := range entries {
		isHealthy := srv.Checks.AggregatedStatus() == api.HealthMaint
		services = append(services, &Service{
			id:        srv.ID,
			address:   srv.Address,
			isHealthy: isHealthy,
		})
	}
	return services, nil
}

func (c *client) RegisterService(serviceName, address string, port int) error {
	serviceId := uuid.New().String()
	registrationReq := &api.AgentServiceRegistration{
		ID:      serviceId,
		Name:    serviceName,
		Address: address,
		Port:    port,
		Check:   c.cfg.Health.toApiConfig(),
	}
	if err := c.client.Agent().ServiceRegister(registrationReq); err != nil {
		return err
	}
	return nil
}
