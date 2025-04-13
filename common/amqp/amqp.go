package amqp

import (
	"context"
	amqp "github.com/rabbitmq/amqp091-go"
	conn "github.com/ykhdr/crack-hash/common/amqp/connection"
)

func Dial(ctx context.Context, cfg *Config) (*conn.Connection, error) {
	opts := amqp.Config{
		SASL: []amqp.Authentication{
			&amqp.PlainAuth{
				Username: cfg.Username,
				Password: cfg.Password,
			},
		},
	}
	return conn.NewConnection(ctx, cfg.URI, opts, cfg.ReconnectTimeout)
}
