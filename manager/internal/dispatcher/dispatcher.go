package dispatcher

import (
	"context"
	"encoding/xml"
	"fmt"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/ykhdr/crack-hash/common/amqp"
	amqpconn "github.com/ykhdr/crack-hash/common/amqp/connection"
	"github.com/ykhdr/crack-hash/common/amqp/consumer"
	"github.com/ykhdr/crack-hash/common/amqp/publisher"
	"github.com/ykhdr/crack-hash/common/consul"
	"github.com/ykhdr/crack-hash/manager/config"
	"github.com/ykhdr/crack-hash/manager/internal/messages/request"
	"github.com/ykhdr/crack-hash/manager/internal/store/requeststore"
	"github.com/ykhdr/crack-hash/manager/pkg/api"
	"github.com/ykhdr/crack-hash/manager/pkg/messages"
	"github.com/ykhdr/crack-hash/worker/pkg/worker"
	"net/http"
	"sync"
	"time"
)

var (
	QueueFullErr              = errors.New("request queue is full")
	RequestNotFoundErr        = errors.New("request not found")
	RequestAlreadyCanceledErr = errors.New("request is already canceled")
)

var ()

type Dispatcher struct {
	l                zerolog.Logger
	requestTimeout   time.Duration
	reconnectTimeout time.Duration
	healthTimeout    time.Duration
	consulClient     consul.Client
	requestStore     requeststore.RequestStore

	amqpConn      *amqpconn.Connection
	amqpPublisher publisher.Publisher[messages.CrackHashManagerRequest]
	amqpConsumer  consumer.Consumer

	publisherCfg *publisher.Config
	consumerCfg  *consumer.Config

	reqQueueLock    sync.RWMutex
	requestC        chan *request.CrackRequest
	dispatchTimeout time.Duration
}

func NewDispatcher(
	cfg *config.DispatcherConfig,
	amqpCfg *amqp.Config,
	consulClient consul.Client,
	requestStore requeststore.RequestStore,
	amqpConn *amqpconn.Connection,
) *Dispatcher {

	return &Dispatcher{
		requestTimeout: cfg.RequestTimeout,
		healthTimeout:  cfg.HealthTimeout,
		consulClient:   consulClient,
		requestStore:   requestStore,
		amqpConn:       amqpConn,
		publisherCfg: amqpCfg.PublisherConfig.ToPublisherConfig(
			xml.Marshal,
			"application/xml",
		),
		consumerCfg: amqpCfg.ConsumerConfig.ToConsumerConfig(
			xml.Unmarshal,
			"",
			false,
			false,
			false,
			false,
			make(map[string]any),
		),
		requestC:        make(chan *request.CrackRequest, cfg.RequestQueueSize),
		dispatchTimeout: cfg.DispatchTimeout,
		l: log.With().
			Str("domain", "dispatcher").
			Logger(),
	}
}

func (s *Dispatcher) Start(ctx context.Context) error {
	s.l.Info().Msg("Dispatcher is running")
	var ch *amqpconn.Channel
	var err error
	for {
		ch, err = s.amqpConn.Channel(ctx)
		if err != nil {
			s.l.Error().Err(err).Msg("Error create channel, retrying in 1 second")
			time.Sleep(s.reconnectTimeout)
			continue
		}
		break
	}
	s.amqpPublisher = publisher.New[messages.CrackHashManagerRequest](ch, s.publisherCfg)
	s.amqpConsumer = consumer.New(ch, s.handle, s.consumerCfg)
	go s.amqpConsumer.Subscribe(ctx)
	for {
		select {
		case req := <-s.requestC:
			s.handleRequest(ctx, req)
		case <-ctx.Done():
			s.l.Debug().Msg("context done")
			return ctx.Err()
		}
	}
}

func (s *Dispatcher) handleRequest(ctx context.Context, req *request.CrackRequest) {
	if req == nil {
		return
	}
	reqInfo := &request.Info{
		ID:        req.ID,
		Status:    request.StatusNew,
		Request:   req.Request,
		CreatedAt: req.CreatedAt,
		FoundData: make([]string, 0),
	}
	services, err := s.consulClient.HealthServices(worker.ServiceName)
	if err != nil {
		s.l.Error().Err(err).Any("request-id", req.ID).Msg("Error get health services")
		reqInfo.Status = request.StatusError
		reqInfo.ErrorReason = "Error getting health workers: " + err.Error()
	} else if len(services) == 0 {
		s.l.Warn().Any("request-id", req.ID).Msg("No services found")
		reqInfo.Status = request.StatusError
		reqInfo.ErrorReason = "No services found"
	}
	reqInfo.ServiceCount = len(services)
	reqInfo.Services = services
	s.requestStore.Save(reqInfo)
	if reqInfo.Status != request.StatusError {
		s.sendRequests(ctx, reqInfo)
	}
}

func (s *Dispatcher) sendRequests(ctx context.Context, reqInfo *request.Info) {
	go s.dispatchTasksToWorkers(ctx, s.amqpPublisher, reqInfo)
	go s.startCheckRequestStatus(ctx, reqInfo.ID, s.healthTimeout)
}

func (s *Dispatcher) dispatchTasksToWorkers(
	ctx context.Context,
	pub publisher.Publisher[messages.CrackHashManagerRequest],
	reqInfo *request.Info) {
	services := reqInfo.Services
	partCount := len(services)
	for partNumber := 0; partNumber < partCount; partNumber++ {
		reqXml := &messages.CrackHashManagerRequest{
			RequestId:  string(reqInfo.ID),
			PartNumber: partNumber,
			PartCount:  partCount,
			Hash:       reqInfo.Request.Hash,
			MaxLength:  reqInfo.Request.MaxLength,
			Alphabet: messages.Alphabet{
				Symbols: generateAlphabet(),
			},
		}
		if err := pub.SendMessage(ctx, reqXml, publisher.Persistent, true, false); err != nil {
			s.l.Warn().Err(err).Msg("Can't send request to worker")
			reqInfo.Status = request.StatusError
			reqInfo.ErrorReason = "cant send request to worker: " + err.Error()
			s.requestStore.Save(reqInfo)
			return
		}
	}
	s.requestStore.UpdateStatus(reqInfo.ID, request.StatusInProgress)
	time.AfterFunc(s.requestTimeout, func() {
		req, exists := s.requestStore.Get(reqInfo.ID)
		if exists && req.Status == request.StatusInProgress {
			s.l.Warn().Any("request-id", req.ID).Msg("Request canceled by timeout")
			req.Status = request.StatusError
			req.ErrorReason = "Request canceled by timeout"
			s.requestStore.Save(req)
		}
	})
}

//func (s *Dispatcher) sendRequestToWorker(
//	ctx context.Context,
//	pub publisher.Publisher[messages.CrackHashManagerRequest],
//	reqXml messages.CrackHashManagerRequest,
//	service *consul.Service,
//) error {
//	bytesToSend, err := xml.Marshal(reqXml)
//	if err != nil {
//		s.l.Warn().Err(err).Msg("Failed to marshal request")
//		return errors.New("failed to marshal request XML")
//	}
//	resp, err := http.Post(fmt.Sprintf("%s/%s", service.Url(), "internal/api/worker/hash/crack/task"), "application/xml",
//		io.NopCloser(bytes.NewReader(bytesToSend)),
//	)
//	if err != nil {
//		s.l.Warn().Err(err).Msg("Failed to send request to worker")
//		return errors.New("failed to send request to worker")
//	}
//	defer func(Body io.ReadCloser) { _ = Body.Close() }(resp.Body)
//	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
//		s.l.Warn().Str("status", resp.Status).Int("code", resp.StatusCode).Msg("Wrong status code")
//		return errors.New("worker responded with status code: " + resp.Status)
//	}
//	s.l.Debug().Msg("Request sent to worker successfully")
//	return nil
//}

func (s *Dispatcher) handle(_ context.Context, data *messages.CrackHashWorkerResponse, delivery amqp091.Delivery) error {
	s.l.Debug().Any("request-id", data.RequestId).Any("found", data.Found).Msg("Handle worker response")
	if err := delivery.Ack(false); err != nil {
		s.l.Error().Err(err).Msg("Error acknowledging message")
		return errors.Wrap(err, "failed ack message")
	}
	reqInfo, exists := s.requestStore.Get(request.Id(data.RequestId))
	if !exists {
		s.l.Warn().Str("request-id", data.RequestId).Msg("Failed to find request store")
		return RequestNotFoundErr
	}
	if reqInfo.Status == request.StatusError || reqInfo.ReadyServiceCount+reqInfo.FailedServiceCount == reqInfo.ServiceCount {
		s.l.Warn().Str("request-id", data.RequestId).Msg("Request is already canceled")
		return RequestAlreadyCanceledErr
	}
	reqInfo.FoundData = append(reqInfo.FoundData, data.Found...)
	reqInfo.ReadyServiceCount++
	reqInfo.UpdateStatus()
	s.requestStore.Save(reqInfo)
	return nil
}

func generateAlphabet() []string {
	alpha := []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	result := make([]string, len(alpha))
	for i, r := range alpha {
		result[i] = string(r)
	}
	return result
}

func (s *Dispatcher) DispatchRequest(apiReq *api.CrackRequest) (request.Id, error) {
	requestId, _ := uuid.NewUUID()
	reqId := request.Id(requestId.String())
	req := &request.CrackRequest{
		ID:        reqId,
		Request:   apiReq,
		CreatedAt: time.Now(),
	}
	s.reqQueueLock.Lock()
	defer s.reqQueueLock.Unlock()
	select {
	case s.requestC <- req:
		return reqId, nil
	case <-time.After(s.dispatchTimeout):
		return "", QueueFullErr
	}
}

func (s *Dispatcher) startCheckRequestStatus(ctx context.Context, reqId request.Id, timeout time.Duration) {
	ticker := time.NewTicker(timeout)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.l.Debug().Msg("check request status cancelled")
			return
		case <-ticker.C:
			//todo добавить отдельный канал на то что запрос завершился
			reqInfo, exists := s.requestStore.Get(reqId)
			if !exists {
				s.l.Warn().Any("request-id", reqId).Msg("Check request error: Request does not exist")
				return
			}
			if reqInfo.Status != request.StatusInProgress && reqInfo.Status != request.StatusNew {
				return
			}
			services := reqInfo.Services
			for _, srv := range services {
				resp, err := http.Get(fmt.Sprintf("%s/api/health", srv.Url()))
				//todo здесь нужно перенаправлять запрос другому воркеру
				if err != nil {
					s.l.Warn().Err(err).Msg("Check request error:Failed to send health request to worker")
					reqInfo.FailedServiceCount++
					reqInfo.ErrorReason += fmt.Sprintf("Failed to send health request to worker %s\n", srv.Id())
					reqInfo.UpdateStatus()
					s.requestStore.Save(reqInfo)
					return
				}
				if resp.StatusCode != http.StatusOK {
					s.l.Warn().
						Str("status", resp.Status).Int("code", resp.StatusCode).Str("worker-id", srv.Id()).
						Msg("Check request error:Failed to send health request to worker")
					reqInfo.FailedServiceCount++
					reqInfo.ErrorReason += fmt.Sprintf("Wrong worker %s health status code\n", srv.Id())
					reqInfo.UpdateStatus()
					s.requestStore.Save(reqInfo)
					return
				}
			}
		}

	}
}
