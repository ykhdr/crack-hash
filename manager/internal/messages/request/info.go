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
	ID                 Id                `bson:"_id"`
	Status             Status            `bson:"status"`
	Request            *api.CrackRequest `bson:"request"`
	FoundData          []string          `bson:"found_data"`
	CreatedAt          time.Time         `bson:"created_at"`
	ServiceCount       int               `bson:"service_count"`
	ReadyServiceCount  int               `bson:"ready_service_count"`
	FailedServiceCount int               `bson:"failed_service_count"`
	ErrorReason        string            `bson:"error_reason"`
	Services           []*consul.Service `bson:"-"`
	CompletedParts     []int             `bson:"completed_parts"`
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
		CompletedParts:     r.CompletedParts,
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
