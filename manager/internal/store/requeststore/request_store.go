package requeststore

import (
	"github.com/ykhdr/crack-hash/manager/internal/messages/request"
	"sync"
)

type RequestStore interface {
	Get(id request.Id) (*request.Info, bool)
	Save(req *request.Info)
	Delete(id request.Id)
	UpdateStatus(id request.Id, status request.Status)
}

type requestStore struct {
	data map[request.Id]*request.Info
	m    sync.RWMutex
}

func NewRequestStore() RequestStore {
	return &requestStore{
		data: make(map[request.Id]*request.Info),
	}
}

func (s *requestStore) Get(id request.Id) (*request.Info, bool) {
	s.m.RLock()
	defer s.m.RUnlock()
	req, exists := s.data[id]
	if !exists {
		return nil, false
	}
	return req.Copy(), true
}

func (s *requestStore) Save(req *request.Info) {
	s.m.Lock()
	defer s.m.Unlock()
	s.data[req.ID] = req.Copy()
}

func (s *requestStore) Delete(id request.Id) {
	s.m.Lock()
	defer s.m.Unlock()
	delete(s.data, id)
}

func (s *requestStore) UpdateStatus(id request.Id, status request.Status) {
	s.m.Lock()
	defer s.m.Unlock()
	req, exists := s.data[id]
	if !exists {
		return
	}
	req.Status = status
}
