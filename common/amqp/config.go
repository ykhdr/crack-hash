package amqp

import (
	"github.com/ykhdr/crack-hash/common/amqp/consumer"
	"github.com/ykhdr/crack-hash/common/amqp/publisher"
	"time"
)

type Config struct {
	URI              string           `kdl:"uri"`
	Username         string           `kdl:"username"`
	Password         string           `kdl:"password"`
	ReconnectTimeout time.Duration    `kdl:"reconnect-timeout"`
	PublisherConfig  *PublisherConfig `kdl:"publisher"`
	ConsumerConfig   *ConsumerConfig  `kdl:"consumer"`
}

type PublisherConfig struct {
	Exchange   string `kdl:"exchange"`
	RoutingKey string `kdl:"routing-key"`
}

func (p *PublisherConfig) ToPublisherConfig(
	marshal publisher.Marshal,
	contentType string,
) *publisher.Config {
	return &publisher.Config{
		Exchange:    p.Exchange,
		RoutingKey:  p.RoutingKey,
		Marshal:     marshal,
		ContentType: contentType,
	}
}

type ConsumerConfig struct {
	Queue string `kdl:"queue"`
}

func (c *ConsumerConfig) ToConsumerConfig(
	unmarshal consumer.Unmarshal,
	consumerS string,
	autoAck bool,
	exclusive bool,
	noLocal bool,
	noWait bool,
	args map[string]any,
) *consumer.Config {
	return &consumer.Config{
		Unmarshal: unmarshal,
		Queue:     c.Queue,
		Consumer:  consumerS,
		AutoAck:   autoAck,
		Exclusive: exclusive,
		NoLocal:   noLocal,
		NoWait:    noWait,
		Args:      args,
	}
}
