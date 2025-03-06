package service

import (
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/ykhdr/crack-hash/common"
	"github.com/ykhdr/crack-hash/common/api"
	"github.com/ykhdr/crack-hash/common/consul"
	"github.com/ykhdr/crack-hash/manager/config"
	"github.com/ykhdr/crack-hash/manager/requests"
	"io"
	log "log/slog"
	"net/http"
	"sync"
	"time"
)

var (
	ErrorQueueFull = errors.New("request queue is full")
)

var (
	m               sync.RWMutex
	requestC        chan *crackRequest
	dispatchTimeout time.Duration
)

type Dispatcher struct {
	requestTimeout time.Duration
	healthTimeout  time.Duration
	consulClient   consul.Client
}

func NewDispatcher(cfg *config.DispatcherConfig, consulClient consul.Client) *Dispatcher {
	m.Lock()
	defer m.Unlock()
	requestC = make(chan *crackRequest, cfg.RequestQueueSize)
	dispatchTimeout = cfg.DispatchTimeout
	return &Dispatcher{
		requestTimeout: cfg.RequestTimeout,
		healthTimeout:  cfg.HealthTimeout,
		consulClient:   consulClient,
	}
}

func (s *Dispatcher) Start() {
	log.Info("Dispatcher is running")
	for {
		req := <-requestC
		s.handleRequest(req)
	}
}

func (s *Dispatcher) handleRequest(req *crackRequest) {
	if req == nil {
		return
	}
	reqInfo := &RequestInfo{
		ID:        req.ID,
		Status:    StatusNew,
		Request:   req.Request,
		CreatedAt: req.CreatedAt,
		FoundData: make([]string, 0),
	}
	services, err := s.consulClient.HealthServices(common.WorkerService)
	if err != nil {
		log.Error("Error getting health services", "reqID", req.ID, "err", err)
		reqInfo.Status = StatusError
		reqInfo.ErrorReason = "Error getting health workers: " + err.Error()
	} else if len(services) == 0 {
		log.Warn("No services found", "reqID", req.ID)
		reqInfo.Status = StatusError
		reqInfo.ErrorReason = "No services found"
	}
	reqInfo.ServiceCount = len(services)
	reqInfo.Services = services
	GetRequestStore().Save(reqInfo)
	if reqInfo.Status != StatusError {
		go s.dispatchTasksToWorkers(reqInfo)
		go startCheckRequestStatus(reqInfo.ID, s.healthTimeout)
	}
}

func (s *Dispatcher) dispatchTasksToWorkers(reqInfo *RequestInfo) {
	services := reqInfo.Services
	partCount := len(services)
	for partNumber := 0; partNumber < partCount; partNumber++ {
		reqXml := api.CrackHashManagerRequest{
			RequestId:  string(reqInfo.ID),
			PartNumber: partNumber,
			PartCount:  partCount,
			Hash:       reqInfo.Request.Hash,
			MaxLength:  reqInfo.Request.MaxLength,
			Alphabet: api.Alphabet{
				Symbols: generateAlphabet(),
			},
		}
		if err := sendRequestToWorker(reqXml, services[partNumber]); err != nil {
			reqInfo.Status = StatusError
			reqInfo.ErrorReason = "cant send request to worker: " + err.Error()
			GetRequestStore().Save(reqInfo)
			return
		}
	}
	GetRequestStore().UpdateStatus(reqInfo.ID, StatusInProgress)
	time.AfterFunc(s.requestTimeout, func() {
		req, exists := GetRequestStore().Get(reqInfo.ID)
		if exists && req.Status == StatusInProgress {
			log.Warn("Request canceled by timeout", "reqID", reqInfo.ID)
			info, _ := GetRequestStore().Get(reqInfo.ID)
			info.Status = StatusError
			info.ErrorReason = "Request canceled by timeout"
			GetRequestStore().Save(info)
		}
	})
}

func sendRequestToWorker(reqXml api.CrackHashManagerRequest, service *consul.Service) error {
	bytesToSend, err := xml.Marshal(reqXml)
	if err != nil {
		log.Warn("Failed to marshal request", "err", err)
		return errors.New("failed to marshal request XML")
	}
	resp, err := http.Post(fmt.Sprintf("%s/%s", service.Url(), "internal/api/worker/hash/crack/task"), "application/xml",
		io.NopCloser(common.NewBytesReader(bytesToSend)),
	)
	if err != nil {
		log.Warn("Failed to send request to worker", "err", err)
		return errors.New("failed to send request to worker")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		log.Warn("Wrong status code", "status", resp.Status, "statusCode", resp.StatusCode)
		return errors.New("worker responded with status code: " + resp.Status)
	}
	log.Debug("Request sent to worker successfully")
	return nil
}

func generateAlphabet() []string {
	// 36 символов: a-z, 0-9
	alpha := []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	result := make([]string, len(alpha))
	for i, r := range alpha {
		result[i] = string(r)
	}
	return result
}

func DispatchRequest(req *requests.CrackRequest) (RequestId, error) {
	m.RLock()
	defer m.RUnlock()
	requestId, _ := uuid.NewUUID()
	reqId := RequestId(requestId.String())
	request := &crackRequest{
		ID:        reqId,
		Request:   req,
		CreatedAt: time.Now(),
	}
	select {
	case requestC <- request:
		return reqId, nil
	case <-time.After(dispatchTimeout):
		return "", ErrorQueueFull
	}
}

func startCheckRequestStatus(reqId RequestId, timeout time.Duration) {
	ticker := time.NewTicker(timeout)
	defer ticker.Stop()
	for {
		<-ticker.C
		reqInfo, exists := GetRequestStore().Get(reqId)
		if !exists {
			log.Warn("Check request error: Request does not exist", "reqId", reqId)
			ticker.Stop()
			return
		}
		services := reqInfo.Services
		for _, srv := range services {
			resp, err := http.Get(fmt.Sprintf("%s/api/health", srv.Url()))
			if err != nil {
				log.Warn("Check request error:Failed to send health request to worker", "err", err)
				reqInfo.FailedServiceCount++
				reqInfo.ErrorReason += fmt.Sprintf("Failed to send health request to worker %s\n", srv.Id())
				checkOnReady(reqInfo)
				GetRequestStore().Save(reqInfo)
				return
			}
			if resp.StatusCode != http.StatusOK {
				log.Warn("Check request error: Wrong worker health status code", "workerId", srv.Id(),
					"status", resp.Status, "statusCode", resp.StatusCode)
				reqInfo.FailedServiceCount++
				reqInfo.ErrorReason += fmt.Sprintf("Wrong worker %s health status code\n", srv.Id())
				checkOnReady(reqInfo)
				GetRequestStore().Save(reqInfo)
				return
			}
		}
	}
}
