package dispatcher

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/ykhdr/crack-hash/common/bytes"
	"github.com/ykhdr/crack-hash/common/consul"
	"github.com/ykhdr/crack-hash/manager/config"
	"github.com/ykhdr/crack-hash/manager/internal/messages/request"
	"github.com/ykhdr/crack-hash/manager/internal/store/requeststore"
	"github.com/ykhdr/crack-hash/manager/pkg/api"
	"github.com/ykhdr/crack-hash/manager/pkg/messages"
	"github.com/ykhdr/crack-hash/worker/pkg/worker"
	"io"
	"net/http"
	"sync"
	"time"
)

var (
	ErrorQueueFull = errors.New("request queue is full")
)

var (
	m               sync.RWMutex
	requestC        chan *request.CrackRequest
	dispatchTimeout time.Duration
)

type Dispatcher struct {
	l              zerolog.Logger
	requestTimeout time.Duration
	healthTimeout  time.Duration
	consulClient   consul.Client
	requestStore   requeststore.RequestStore
}

func NewDispatcher(
	cfg *config.DispatcherConfig,
	l zerolog.Logger,
	consulClient consul.Client,
	requestStore requeststore.RequestStore,
) *Dispatcher {
	m.Lock()
	defer m.Unlock()
	requestC = make(chan *request.CrackRequest, cfg.RequestQueueSize)
	dispatchTimeout = cfg.DispatchTimeout
	return &Dispatcher{
		requestTimeout: cfg.RequestTimeout,
		healthTimeout:  cfg.HealthTimeout,
		consulClient:   consulClient,
		requestStore:   requestStore,
		l: l.With().
			Str("domain", "dispatcher").
			Logger(),
	}
}

func (s *Dispatcher) Start(ctx context.Context) error {
	s.l.Info().Msg("Dispatcher is running")
	for {
		select {
		case req := <-requestC:
			s.handleRequest(req)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *Dispatcher) handleRequest(req *request.CrackRequest) {
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
		go s.dispatchTasksToWorkers(reqInfo)
		go s.startCheckRequestStatus(reqInfo.ID, s.healthTimeout)
	}
}

func (s *Dispatcher) dispatchTasksToWorkers(reqInfo *request.Info) {
	services := reqInfo.Services
	partCount := len(services)
	for partNumber := 0; partNumber < partCount; partNumber++ {
		reqXml := messages.CrackHashManagerRequest{
			RequestId:  string(reqInfo.ID),
			PartNumber: partNumber,
			PartCount:  partCount,
			Hash:       reqInfo.Request.Hash,
			MaxLength:  reqInfo.Request.MaxLength,
			Alphabet: messages.Alphabet{
				Symbols: generateAlphabet(),
			},
		}
		if err := s.sendRequestToWorker(reqXml, services[partNumber]); err != nil {
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

func (s *Dispatcher) sendRequestToWorker(reqXml messages.CrackHashManagerRequest, service *consul.Service) error {
	bytesToSend, err := xml.Marshal(reqXml)
	if err != nil {
		s.l.Warn().Err(err).Msg("Failed to marshal request")
		return errors.New("failed to marshal request XML")
	}
	resp, err := http.Post(fmt.Sprintf("%s/%s", service.Url(), "internal/api/worker/hash/crack/task"), "application/xml",
		io.NopCloser(bytes.NewReader(bytesToSend)),
	)
	if err != nil {
		s.l.Warn().Err(err).Msg("Failed to send request to worker")
		return errors.New("failed to send request to worker")
	}
	defer func(Body io.ReadCloser) { _ = Body.Close() }(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		s.l.Warn().Str("status", resp.Status).Int("code", resp.StatusCode).Msg("Wrong status code")
		return errors.New("worker responded with status code: " + resp.Status)
	}
	s.l.Debug().Msg("Request sent to worker successfully")
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

func DispatchRequest(apiReq *api.CrackRequest) (request.Id, error) {
	m.RLock()
	defer m.RUnlock()
	requestId, _ := uuid.NewUUID()
	reqId := request.Id(requestId.String())
	req := &request.CrackRequest{
		ID:        reqId,
		Request:   apiReq,
		CreatedAt: time.Now(),
	}
	select {
	case requestC <- req:
		return reqId, nil
	case <-time.After(dispatchTimeout):
		return "", ErrorQueueFull
	}
}

func (s *Dispatcher) startCheckRequestStatus(reqId request.Id, timeout time.Duration) {
	ticker := time.NewTicker(timeout)
	defer ticker.Stop()
	for {
		<-ticker.C
		reqInfo, exists := s.requestStore.Get(reqId)
		if !exists {
			s.l.Warn().Any("request-id", reqId).Msg("Check request error: Request does not exist")
			ticker.Stop()
			return
		}
		services := reqInfo.Services
		for _, srv := range services {
			resp, err := http.Get(fmt.Sprintf("%s/api/health", srv.Url()))
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
