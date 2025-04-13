package hashcrack

import (
	"context"
	"encoding/xml"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	amqpconn "github.com/ykhdr/crack-hash/common/amqp/connection"
	"github.com/ykhdr/crack-hash/common/amqp/consumer"
	"github.com/ykhdr/crack-hash/common/amqp/publisher"
	"github.com/ykhdr/crack-hash/common/consul"
	"github.com/ykhdr/crack-hash/manager/pkg/messages"
	"github.com/ykhdr/crack-hash/worker/config"
	"github.com/ykhdr/crack-hash/worker/internal/hashcrack/strategy"
	"github.com/ykhdr/crack-hash/worker/pkg/worker"
)

type Service struct {
	l            zerolog.Logger
	cfg          *config.WorkerConfig
	consumerCfg  *consumer.Config
	publisherCfg *publisher.Config
	consulClient consul.Client

	crackStrategy strategy.Strategy

	amqpConn      *amqpconn.Connection
	amqpConsumer  consumer.Consumer
	amqpPublisher publisher.Publisher[messages.CrackHashWorkerResponse]
}

func NewService(
	cfg *config.WorkerConfig,
	consulClient consul.Client,
	amqpConn *amqpconn.Connection,
) *Service {
	crackStrategy := strategy.NewStrategy(strategy.ParseStrategyName(cfg.Strategy))
	return &Service{
		cfg:           cfg,
		consulClient:  consulClient,
		crackStrategy: crackStrategy,
		amqpConn:      amqpConn,
		consumerCfg: cfg.AmqpConfig.ConsumerConfig.ToConsumerConfig(
			xml.Unmarshal,
			"",
			false,
			false,
			false,
			false,
			nil,
		),
		publisherCfg: cfg.AmqpConfig.PublisherConfig.ToPublisherConfig(
			xml.Marshal,
			"application/xml",
		),
		l: log.With().
			Str("domain", "hashcrack").
			Logger(),
	}
}

func (s *Service) Start(ctx context.Context) error {
	err := s.consulClient.RegisterService(worker.ServiceName, s.cfg.Address, s.cfg.Port)
	if err != nil {
		s.l.Warn().Err(err).Msgf("Error register service in consul")
		return errors.Wrap(err, "error register service in consul")
	}
	ch, err := s.amqpConn.Channel(ctx)
	if err != nil {
		s.l.Warn().Err(err).Msgf("Error create amqp channel")
		return errors.Wrap(err, "error create amqp channel")
	}
	s.amqpConsumer = consumer.New(ch, s.receive, s.consumerCfg)
	s.amqpPublisher = publisher.New[messages.CrackHashWorkerResponse](ch, s.publisherCfg)
	s.l.Debug().Msg("Hashcrack service is running")
	s.amqpConsumer.Subscribe(ctx)
	return nil
}

func (s *Service) receive(ctx context.Context, data *messages.CrackHashManagerRequest, d amqp.Delivery) error {
	resp := s.crackTask(data)
	if err := d.Ack(false); err != nil {
		return errors.Wrap(err, "error ack delivery")
	}
	return s.amqpPublisher.SendMessage(ctx, resp, publisher.Persistent, false, false)
}

func (s *Service) crackTask(req *messages.CrackHashManagerRequest) *messages.CrackHashWorkerResponse {
	s.l.Debug().
		Str("req-id", req.RequestId).
		Int("part-number", req.PartNumber).
		Int("part-count", req.PartCount).
		Str("hash", req.Hash).
		Int("max-length", req.MaxLength).
		Msg("cracking task")
	result := s.crackStrategy.CrackMd5(req)
	return &messages.CrackHashWorkerResponse{
		Id:        uuid.NewString(),
		RequestId: req.RequestId,
		Found:     result.Found(),
	}
}
