package publisher

import (
	"context"
	"encoding/json"
	"github.com/pkg/errors"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/ykhdr/crack-hash/common/amqp/connection"
)

type DeliveryMode uint8

const (
	Transient  DeliveryMode = 1
	Persistent DeliveryMode = 2
)

type Marshal func(any) ([]byte, error)
type Config struct {
	Exchange    string
	RoutingKey  string
	Marshal     Marshal
	ContentType string
}

type Publisher[T any] interface {
	SendMessage(ctx context.Context, message *T, mode DeliveryMode, mandatory, immediate bool) error
}

type publisher[T any] struct {
	cfg         *Config
	ch          *connection.Channel
	marshal     Marshal
	contentType string
	l           zerolog.Logger
}

func New[T any](ch *connection.Channel, config *Config) Publisher[T] {
	if config.Marshal == nil {
		config.Marshal = json.Marshal
	}
	if config.ContentType == "" {
		config.ContentType = "application/json"
	}
	pub := &publisher[T]{
		cfg:         config,
		ch:          ch,
		marshal:     config.Marshal,
		contentType: config.ContentType,
		l: log.With().
			Str("component", "amqp-publisher").
			Type("type", *new(T)).
			Str("exchange", config.Exchange).
			Str("routing-key", config.RoutingKey).
			Logger(),
	}
	return pub
}

func (p *publisher[T]) SendMessage(ctx context.Context, message *T, mode DeliveryMode, mandatory, immediate bool) error {
	p.l.Debug().Msg("send message")
	body, err := p.marshal(message)
	if err != nil {
		p.l.Error().Err(err).Stack().Msg("failed to marshal message")
		return errors.Wrap(err, "failed to marshal message")
	}
	amqpMsg := p.buildMessage(body, mode)
	err = p.sendMessage(ctx, mandatory, immediate, amqpMsg)
	if err != nil {
		p.l.Error().Err(err).Stack().Msg("failed to send message")
		return errors.Wrap(err, "failed to send message")
	}
	return nil
}

func (p *publisher[T]) sendMessage(ctx context.Context, mandatory, immediate bool, ampqMsg *amqp.Publishing) error {
	if err := p.ch.Publish(
		ctx,
		p.cfg.Exchange,
		p.cfg.RoutingKey,
		mandatory,
		immediate,
		*ampqMsg,
	); err != nil {
		return errors.Wrap(err, "failed to publish a message")
	}
	return nil
}

func (p *publisher[T]) buildMessage(body []byte, mode DeliveryMode) *amqp.Publishing {
	return &amqp.Publishing{
		DeliveryMode: uint8(mode),
		ContentType:  p.contentType,
		Body:         body,
	}
}
