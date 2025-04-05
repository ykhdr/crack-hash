package request

import (
	"github.com/ykhdr/crack-hash/common/consul"
	"github.com/ykhdr/crack-hash/manager/pkg/api"
	"time"
)

type Status string

const (
	StatusNew          Status = "NEW"
	StatusInProgress   Status = "IN_PROGRESS"
	StatusReady        Status = "READY"
	StatusError        Status = "ERROR"
	StatusPartialReady Status = "PARTIAL_READY"
)

type Id string

type CrackRequest struct {
	ID        Id
	Request   *api.CrackRequest
	CreatedAt time.Time
}

type Info struct {
	ID                 Id
	Status             Status
	Request            *api.CrackRequest
	FoundData          []string
	CreatedAt          time.Time
	ServiceCount       int
	ReadyServiceCount  int
	FailedServiceCount int
	ErrorReason        string
	Services           []*consul.Service
}

func (r *Info) Copy() *Info {
	return &Info{
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

func (r *Info) UpdateStatus() {
	if r.FailedServiceCount == r.ServiceCount {
		r.Status = StatusError
		return
	}
	if r.ReadyServiceCount+r.FailedServiceCount == r.ServiceCount {
		if r.FailedServiceCount > 0 {
			r.Status = StatusPartialReady
		} else {
			r.Status = StatusReady
		}
	}
}
