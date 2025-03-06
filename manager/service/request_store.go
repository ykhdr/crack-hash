package service

import (
	"github.com/ykhdr/crack-hash/common/consul"
	"github.com/ykhdr/crack-hash/manager/requests"
	"sync"
	"time"
)

type RequestStatus string

const (
	StatusNew          RequestStatus = "NEW"
	StatusInProgress   RequestStatus = "IN_PROGRESS"
	StatusReady        RequestStatus = "READY"
	StatusError        RequestStatus = "ERROR"
	StatusPartialReady RequestStatus = "PARTIAL_READY"
)

type RequestId string

type crackRequest struct {
	ID        RequestId
	Request   *requests.CrackRequest
	CreatedAt time.Time
}

type RequestInfo struct {
	ID                 RequestId
	Status             RequestStatus
	Request            *requests.CrackRequest
	FoundData          []string
	CreatedAt          time.Time
	ServiceCount       int
	ReadyServiceCount  int
	FailedServiceCount int
	ErrorReason        string
	Services           []*consul.Service
}

func (r *RequestInfo) Copy() *RequestInfo {
	return &RequestInfo{
		ID:                 r.ID,
		Status:             r.Status,
		Request:            r.Request,
		FoundData:          r.FoundData,
		CreatedAt:          r.CreatedAt,
		ServiceCount:       r.ServiceCount,
		ReadyServiceCount:  r.ReadyServiceCount,
		ErrorReason:        r.ErrorReason,
		Services:           r.Services,
		FailedServiceCount: r.FailedServiceCount,
	}
}

type RequestStore struct {
	data map[RequestId]*RequestInfo
	m    sync.RWMutex
}

var reqStore = &RequestStore{data: map[RequestId]*RequestInfo{}}

func GetRequestStore() *RequestStore {
	return reqStore
}

func (r *RequestStore) Get(id RequestId) (*RequestInfo, bool) {
	m.RLock()
	defer m.RUnlock()
	req, exists := r.data[id]
	if !exists {
		return nil, false
	}
	return req.Copy(), true
}

func (r *RequestStore) Save(req *RequestInfo) {
	m.Lock()
	defer m.Unlock()
	r.data[req.ID] = req.Copy()
}

func (r *RequestStore) Delete(id RequestId) {
	m.Lock()
	defer m.Unlock()
	delete(r.data, id)
}

func (r *RequestStore) UpdateStatus(id RequestId, status RequestStatus) {
	m.Lock()
	defer m.Unlock()
	req, exists := r.data[id]
	if !exists {
		return
	}
	req.Status = status
}
