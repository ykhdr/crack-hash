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
	"github.com/ykhdr/crack-hash/manager/internal/store/respstore"
	"github.com/ykhdr/crack-hash/manager/pkg/api"
	"github.com/ykhdr/crack-hash/manager/pkg/messages"
	"github.com/ykhdr/crack-hash/worker/pkg/worker"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"net/http"
	"slices"
	"sync"
	"time"
)

var (
	QueueFullErr              = errors.New("request queue is full")
	RequestNotFoundErr        = errors.New("request not found")
	RequestAlreadyCanceledErr = errors.New("request is already canceled")
	NilRequestErr             = fmt.Errorf("request is nil")
	SaveRequestErr            = fmt.Errorf("save request error")
)

type Dispatcher struct {
	l                zerolog.Logger
	requestTimeout   time.Duration
	reconnectTimeout time.Duration
	healthTimeout    time.Duration
	consulClient     consul.Client

	requestStore  requeststore.RequestStore
	responseStore respstore.ResponseStore
	requestLock   sync.RWMutex
	mongoClient   *mongo.Client

	amqpConn      *amqpconn.Connection
	amqpPublisher publisher.Publisher[messages.CrackHashManagerRequest]
	amqpConsumer  consumer.Consumer

	publisherCfg *publisher.Config
	consumerCfg  *consumer.Config

	requestC        chan *request.CrackRequest
	dispatchTimeout time.Duration
}

func NewDispatcher(
	cfg *config.DispatcherConfig,
	amqpCfg *amqp.Config,
	consulClient consul.Client,
	requestStore requeststore.RequestStore,
	responseStore respstore.ResponseStore,
	mongoClient *mongo.Client,
	amqpConn *amqpconn.Connection,
) *Dispatcher {
	return &Dispatcher{
		requestTimeout: cfg.RequestTimeout,
		healthTimeout:  cfg.HealthTimeout,
		consulClient:   consulClient,
		requestStore:   requestStore,
		responseStore:  responseStore,
		mongoClient:    mongoClient,
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
	go s.routineSavedRequests(ctx)
	s.amqpConsumer.Subscribe(ctx)
	return nil
}

func (s *Dispatcher) routineSavedRequests(ctx context.Context) {
	s.l.Debug().Msg("Start routine saved requests")
	requests, err := s.requestStore.List(ctx)
	if err != nil {
		s.l.Error().Err(err).Msg("Error listing requests")
		return
	}
	for _, req := range requests {
		switch req.Status {
		case request.StatusNew:
			if err := s.dispatchRequest(ctx, req); err != nil {
				s.l.Error().Err(err).Msg("Error dispatching request")
			}
		case request.StatusInProgress:
			s.l.Debug().Any("request", req).Msgf("Request is in progress")
			s.requestLock.Lock()
			responses, err := s.responseStore.GetByRequestId(ctx, string(req.ID))
			if err != nil {
				s.l.Error().Err(err).Msg("Error getting responses from store")
				s.requestLock.Unlock()
				continue
			}
			for _, resp := range responses {
				if err := s.handleResponse(ctx, resp); err != nil {
					s.l.Error().Err(err).Msg("Error handling response")
				}
			}
			if err = s.responseStore.DeleteByRequestId(ctx, string(req.ID)); err != nil {
				s.l.Error().Err(err).Msg("Error deleting response from store")
				s.requestLock.Unlock()
				continue
			}
			s.requestLock.Unlock()
		}
	}
	s.l.Debug().Msg("Finished routine saved requests")
}

func (s *Dispatcher) handleRequest(ctx context.Context, req *request.CrackRequest) error {
	if req == nil {
		return NilRequestErr
	}
	reqInfo := &request.Info{
		ID:        req.ID,
		Status:    request.StatusNew,
		Request:   req.Request,
		CreatedAt: req.CreatedAt,
		FoundData: make([]string, 0),
	}
	return s.dispatchRequest(ctx, reqInfo)
}

func (s *Dispatcher) dispatchRequest(ctx context.Context, req *request.Info) error {
	services, err := s.consulClient.HealthServices(worker.ServiceName)
	if err != nil {
		s.l.Error().Err(err).Any("request-id", req.ID).Msg("Error get health services")
		req.Status = request.StatusError
		req.ErrorReason = "Error getting health workers: " + err.Error()
	} else if len(services) == 0 {
		s.l.Warn().Any("request-id", req.ID).Msg("No services found")
		req.Status = request.StatusError
		req.ErrorReason = "No services found"
	}
	req.ServiceCount = len(services)
	req.Services = services
	if err := s.requestStore.Save(ctx, req); err != nil {
		s.l.Error().Err(err).Msg("Error saving request")
		return SaveRequestErr
	}
	if req.Status != request.StatusError {
		s.sendRequests(ctx, req)
	}
	return nil
}

func (s *Dispatcher) sendRequests(ctx context.Context, reqInfo *request.Info) {
	go s.dispatchTasksToWorkers(ctx, s.amqpPublisher, reqInfo)
	go s.startCheckRequestStatus(ctx, reqInfo.ID, s.healthTimeout)
}

func (s *Dispatcher) dispatchTasksToWorkers(
	ctx context.Context,
	pub publisher.Publisher[messages.CrackHashManagerRequest],
	reqInfo *request.Info) {
	s.requestLock.Lock()
	defer s.requestLock.Unlock()
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
			reqInfo.ErrorReason = "Can't send request to worker"
			if err := s.requestStore.UpdateStatus(ctx, reqInfo.ID, reqInfo.Status, reqInfo.ErrorReason); err != nil {
				s.l.Error().Err(err).Msg("Error updating request status")
			}
			return
		}
	}
	err := s.requestStore.UpdateStatus(ctx, reqInfo.ID, request.StatusInProgress, "")
	if err != nil {
		s.l.Warn().Err(err).Msg("Can't update status to in-progress")
	}
	//timer := time.AfterFunc(s.requestTimeout, func() {
	//	s.requestLock.Lock()
	//	defer s.requestLock.Unlock()
	//	req, err := s.requestStore.Get(ctx, reqInfo.ID)
	//	if err != nil {
	//		s.l.Error().Err(err).Msg("Can't get request")
	//		reqInfo.Status = request.StatusError
	//		reqInfo.ErrorReason = "Can't get request"
	//		if err := s.requestStore.UpdateStatus(ctx, reqInfo.ID, reqInfo.Status, reqInfo.ErrorReason); err != nil {
	//			s.l.Error().Err(err).Msg("Error updating request status")
	//		}
	//	} else if req.Status == request.StatusInProgress {
	//		s.l.Warn().Any("request-id", req.ID).Msg("Request canceled by timeout")
	//		req.Status = request.StatusError
	//		req.ErrorReason = "Request canceled by timeout"
	//		if err := s.requestStore.UpdateStatus(ctx, reqInfo.ID, reqInfo.Status, reqInfo.ErrorReason); err != nil {
	//			s.l.Error().Err(err).Msg("Error updating request status")
	//		}
	//	}
	//})
}

func (s *Dispatcher) handle(ctx context.Context, data *messages.CrackHashWorkerResponse, delivery amqp091.Delivery) error {
	s.l.Debug().Any("request-id", data.RequestId).Any("found", data.Found).Msg("Handle worker response")
	if data.Id == "" {
		data.Id = uuid.NewString()
	}
	if err := s.responseStore.Save(ctx, data); err != nil {
		s.l.Warn().Err(err).Msg("Error saving response")
		return errors.Wrap(err, "can't save response")
	}
	if err := delivery.Ack(false); err != nil {
		s.l.Warn().Err(err).Msg("Error acknowledging message")
		return errors.Wrap(err, "failed ack message")
	}
	if err := s.handleResponse(ctx, data); err != nil {
		s.l.Warn().Err(err).Msg("Error handling response")
		return errors.Wrap(err, "can't handle response")
	}
	return s.responseStore.DeleteByResponseId(ctx, data.Id)
}

func (s *Dispatcher) handleResponse(ctx context.Context, resp *messages.CrackHashWorkerResponse) error {
	s.requestLock.Lock()
	defer s.requestLock.Unlock()
	reqInfo, err := s.requestStore.Get(ctx, request.Id(resp.RequestId))
	if err != nil {
		s.l.Warn().Str("request-id", resp.RequestId).Msg("Failed to find request store")
		return RequestNotFoundErr
	}
	if reqInfo.Status != request.StatusInProgress || reqInfo.ReadyServiceCount+reqInfo.FailedServiceCount == reqInfo.ServiceCount {
		s.l.Warn().Str("request-id", resp.RequestId).Msg("Request is already canceled")
		return RequestAlreadyCanceledErr
	}
	for _, found := range resp.Found {
		if !slices.Contains(reqInfo.FoundData, found) {
			reqInfo.FoundData = append(reqInfo.FoundData, found)
		}
	}
	reqInfo.ReadyServiceCount++
	reqInfo.UpdateStatus()
	return s.requestStore.Update(ctx, reqInfo)
}

func generateAlphabet() []string {
	alpha := []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	result := make([]string, len(alpha))
	for i, r := range alpha {
		result[i] = string(r)
	}
	return result
}

func (s *Dispatcher) DispatchRequest(ctx context.Context, apiReq *api.CrackRequest) (request.Id, error) {
	requestId, _ := uuid.NewUUID()
	reqId := request.Id(requestId.String())
	req := &request.CrackRequest{
		ID:        reqId,
		Request:   apiReq,
		CreatedAt: time.Now(),
	}
	if err := s.handleRequest(ctx, req); err != nil {
		return "", err
	}
	return reqId, nil
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
			reqInfo, err := s.requestStore.Get(ctx, reqId)
			if err != nil {
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
					s.l.Warn().Err(err).Msg("Check request error: Failed to send health request to worker")
					reqInfo.FailedServiceCount++
					reqInfo.ErrorReason += fmt.Sprintf("Failed to send health request to worker %s\n", srv.Id())
					reqInfo.UpdateStatus()
					if err := s.requestStore.Update(ctx, reqInfo); err != nil {
						s.l.Error().Err(err).Msg("Error updating request status")
					}
					return
				}
				if resp.StatusCode != http.StatusOK {
					s.l.Warn().
						Str("status", resp.Status).Int("code", resp.StatusCode).Str("worker-id", srv.Id()).
						Msg("Check request error:Failed to send health request to worker")
					reqInfo.FailedServiceCount++
					reqInfo.ErrorReason += fmt.Sprintf("Wrong worker %s health status code\n", srv.Id())
					reqInfo.UpdateStatus()
					if err := s.requestStore.Update(ctx, reqInfo); err != nil {
						s.l.Error().Err(err).Msg("Error updating request status")
					}
					return
				}
			}
		}

	}
}

//func (s *Dispatcher) monitorMongo(ctx context.Context) {
//	for {
//		select {
//		case <-ctx.Done():
//			return
//		case <-time.After(time.Second):
//			if err := s.mongoClient.Ping(ctx, nil); err != nil {
//				s.l.Warn().Err(err).Msg("Failed to ping mongo")
//				//todo блокировку тут делать?
//				//s.mongoAvailable
//			}
//		}
//	}
//}
