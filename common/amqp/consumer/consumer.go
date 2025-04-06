package consumer

import (
	"context"
	"encoding/json"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/ykhdr/crack-hash/common/amqp/connection"
	"runtime/debug"
)

type Unmarshal func(data []byte, v any) error

type Handler[T any] func(ctx context.Context, data *T, delivery amqp.Delivery) error

type Config struct {
	Unmarshal Unmarshal
	Queue     string
	Consumer  string
	AutoAck   bool
	Exclusive bool
	NoLocal   bool
	NoWait    bool
	Args      map[string]any
}

type Consumer interface {
	Subscribe(ctx context.Context)
}

type consumer[T any] struct {
	cfg       *Config
	ch        *connection.Channel
	handler   Handler[T]
	unmarshal Unmarshal
	l         zerolog.Logger
}

func New[T any](ch *connection.Channel, handler Handler[T], cfg *Config) Consumer {
	if handler == nil {
		handler = func(context.Context, *T, amqp.Delivery) error { return nil }
	}
	if cfg.Unmarshal == nil {
		cfg.Unmarshal = json.Unmarshal
	}
	c := &consumer[T]{
		ch:        ch,
		handler:   handler,
		cfg:       cfg,
		unmarshal: cfg.Unmarshal,
		l: log.With().
			Str("component", "amqp-consumer").
			Type("type", *new(T)).
			Str("queue", cfg.Queue).
			Logger(),
	}

	return c
}

func (c *consumer[T]) connect(ctx context.Context) <-chan amqp.Delivery {
	return c.ch.Consume(
		ctx,
		c.cfg.Queue,
		c.cfg.Consumer,
		c.cfg.AutoAck,
		c.cfg.Exclusive,
		c.cfg.NoLocal,
		c.cfg.NoWait,
		c.cfg.Args,
	)
}

func (c *consumer[T]) Subscribe(ctx context.Context) {
	msgCh := c.connect(ctx)
	c.l.Debug().Msg("consumer connected")
	for {
		select {
		case <-ctx.Done():
			c.l.Debug().Msg("consumer stopped")
			return

		case d, ok := <-msgCh:
			if !ok {
				if c.ch.IsClosed() {
					return
				}
				c.l.Debug().Msg("consumer closed, try to reconnect")
				msgCh = c.connect(ctx)
				continue
			}

			c.l.Debug().Bytes("body", d.Body).Msg("got new event")
			data := *new(T)
			if err := c.unmarshal(d.Body, &data); err != nil {
				c.l.Error().Err(err).Msg("failed to unmarshal event")
				continue
			}
			c.handle(ctx, &data, d)
		}
	}
}

func (c *consumer[T]) handle(ctx context.Context, data *T, d amqp.Delivery) {
	defer func() {
		if r := recover(); r != nil {
			c.l.Error().Msgf("catch panic: %v\n%s", r, string(debug.Stack()))
		}
	}()
	if err := c.handler(ctx, data, d); err != nil {
		c.l.Error().Err(err).Msg("failed to consume event")
	}
}
