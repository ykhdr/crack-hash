package connection

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ConnAlreadyClosedErr    = errors.New("connection is already closed")
	ChannelAlreadyClosedErr = errors.New("channel is already closed")
)

type Connection struct {
	l    zerolog.Logger
	uri  string
	opts amqp.Config
	conn *amqp.Connection

	reconnectTimeout time.Duration

	reconnectLock sync.RWMutex
	reconnect     atomic.Bool
	closed        atomic.Bool

	cancel context.CancelFunc
}

type Channel struct {
	l    zerolog.Logger
	ch   *amqp.Channel
	conn *Connection

	reconnectTimeout time.Duration

	reconnectLock sync.RWMutex
	reconnect     atomic.Bool
	closed        atomic.Bool

	cancel context.CancelFunc
}

func NewConnection(
	ctx context.Context,
	uri string,
	opts amqp.Config,
	reconnectTimeout time.Duration,
) (*Connection, error) {
	c, err := amqp.DialConfig(uri, opts)
	if err != nil {
		return nil, fmt.Errorf("erro dial amqp connection: %w", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	conn := &Connection{
		uri:              uri,
		opts:             opts,
		conn:             c,
		cancel:           cancel,
		reconnectTimeout: reconnectTimeout,
		l:                log.With().Str("component", "amqp-connection").Logger(),
	}
	go conn.runNotifyWatcher(ctx)
	return conn, nil
}

func (c *Connection) Connection() *amqp.Connection {
	c.reconnectLock.RLock()
	defer c.reconnectLock.RUnlock()
	return c.conn
}

func (c *Connection) Close() error {
	if c.closed.Load() {
		return ConnAlreadyClosedErr
	}
	c.reconnectLock.Lock()
	defer c.reconnectLock.Unlock()
	c.cancel()
	c.closed.Store(true)
	if err := c.conn.Close(); err != nil {
		return errors.Wrap(err, "error close amqp connection")
	}
	return nil
}

func (c *Connection) runNotifyWatcher(ctx context.Context) {
	c.l.Debug().Msg("amqp connection watcher running")
	select {
	case <-ctx.Done():
		c.l.Debug().Msg("watcher stopped")
		return
	case err, ok := <-c.conn.NotifyClose(make(chan *amqp.Error)):
		if !ok {
			c.l.Debug().Msg("watcher stopped")
			return
		}
		c.l.Warn().Err(err).Msg("connection closed, try to reconnect")
		c.reconnectLock.Lock()
		c.reconnect.Store(true)
		for {
			if c.closed.Load() {
				c.l.Debug().Msg("amqp connection closed")
				return
			}
			cc, err := amqp.DialConfig(c.uri, c.opts)
			if err != nil {
				c.l.Warn().Err(err).Msg("amqp connection error")
				time.Sleep(c.reconnectTimeout)
				continue
			}
			c.conn = cc
			break
		}
		c.reconnectLock.Unlock()
		c.reconnect.Store(false)
		c.l.Debug().Msg("amqp connection reconnected")
	}
}

func (c *Connection) Channel(ctx context.Context) (*Channel, error) {
	amqpCh, err := c.conn.Channel()
	if err != nil {
		return nil, errors.Wrap(err, "failed to open channel")
	}
	ctx, cancel := context.WithCancel(ctx)
	ch := &Channel{
		ch:               amqpCh,
		conn:             c,
		reconnectTimeout: c.reconnectTimeout,
		cancel:           cancel,
		l:                log.With().Str("component", "amqp-channel").Logger(),
	}
	go ch.runNotifyWatcher(ctx)
	return ch, nil
}

func (ch *Channel) Channel() *amqp.Channel {
	ch.reconnectLock.RLock()
	defer ch.reconnectLock.RUnlock()
	return ch.ch
}

func (ch *Channel) Close() error {
	if ch.closed.Load() {
		return ChannelAlreadyClosedErr
	}
	ch.cancel()
	ch.closed.Store(true)
	if err := ch.ch.Close(); err != nil {
		return errors.Wrap(err, "failed to close amqp channel")
	}
	return nil
}

func (ch *Channel) IsClosed() bool {
	return ch.closed.Load()
}

func (ch *Channel) Consume(
	ctx context.Context, queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table,
) <-chan amqp.Delivery {
	deliveries := make(chan amqp.Delivery)
	go ch.runConsumer(ctx, deliveries, queue, consumer, autoAck, exclusive, noLocal, noWait, args)
	return deliveries
}

func (ch *Channel) runConsumer(
	ctx context.Context, deliveries chan<- amqp.Delivery, queue, consumer string, autoAck, exclusive, noLocal,
	noWait bool, args amqp.Table,
) {
	for {
		d, err := ch.Channel().ConsumeWithContext(ctx, queue, consumer, autoAck, exclusive, noLocal, noWait, args)
		if err != nil {
			ch.l.Error().Err(err).Msg("failed to consume")
			time.Sleep(ch.reconnectTimeout)
			continue
		}
		for msg := range d {
			deliveries <- msg
		}
		if ch.IsClosed() {
			close(deliveries)
			break
		}
	}
}

func (ch *Channel) Publish(ctx context.Context, exchange string, key string, mandatory bool, immediate bool, msg amqp.Publishing) error {
	if err := ch.Channel().PublishWithContext(ctx, exchange, key, mandatory, immediate, msg); err != nil {
		return errors.Wrap(err, "failed to publish")
	}
	return nil
}

func (ch *Channel) runNotifyWatcher(ctx context.Context) {
	ch.l.Debug().Msg("amqp channel watcher running")
	select {
	case <-ctx.Done():
		ch.l.Debug().Msg("watcher stopped")
		return
	case err, ok := <-ch.ch.NotifyClose(make(chan *amqp.Error)):
		if !ok {
			ch.l.Debug().Msg("watcher stopped")
			return
		}
		ch.l.Warn().Err(err).Msg("connection to channel closed, try to reconnect")
		ch.reconnectLock.Lock()
		ch.reconnect.Store(true)
		for {
			if ch.closed.Load() {
				ch.l.Debug().Msg("amqp channel closed")
				return
			}
			cch, err := ch.conn.Connection().Channel()
			if err != nil {
				ch.l.Warn().Err(err).Msg("amqp channel connection error")
				time.Sleep(ch.reconnectTimeout)
				continue
			}
			ch.ch = cch
			break
		}
		ch.reconnectLock.Unlock()
		ch.reconnect.Store(false)
		ch.l.Debug().Msg("amqp channel reconnected")
	}
}
