package respstore

import (
	"context"
	"github.com/pkg/errors"
	"github.com/ykhdr/crack-hash/manager/pkg/messages"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"sync"
)

const ResponseCollection = "responses"

var NotFoundErr = errors.New("response not found")

type ResponseStore interface {
	Save(ctx context.Context, resp *messages.CrackHashWorkerResponse) error
	GetByRequestId(ctx context.Context, id string) ([]*messages.CrackHashWorkerResponse, error)
	DeleteByRequestId(ctx context.Context, id string) error
	DeleteByResponseId(ctx context.Context, id string) error
}

type responseStore struct {
	data     map[string][]*messages.CrackHashWorkerResponse
	database *mongo.Database
	m        sync.RWMutex
}

func NewResponseStore(database *mongo.Database) ResponseStore {
	return &responseStore{
		database: database,
		data:     make(map[string][]*messages.CrackHashWorkerResponse),
	}
}

func (s *responseStore) Save(ctx context.Context, resp *messages.CrackHashWorkerResponse) error {
	s.m.Lock()
	defer s.m.Unlock()
	resps, exists := s.data[resp.RequestId]
	if !exists {
		if err := s.loadStore(ctx, resp.RequestId); err != nil {
			return err
		}
		resps = s.data[resp.RequestId]
	}
	resps = append(resps, resp)
	s.data[resp.RequestId] = resps
	_, err := s.database.Collection(ResponseCollection).InsertOne(ctx, resp)
	return err
}

func (s *responseStore) GetByRequestId(ctx context.Context, id string) ([]*messages.CrackHashWorkerResponse, error) {
	s.m.RLock()
	defer s.m.RUnlock()
	resps, exists := s.data[id]
	if !exists {
		if err := s.loadStore(ctx, id); err != nil {
			return nil, err
		}
	}
	resps = s.data[id]
	if len(resps) == 0 {
		return nil, NotFoundErr
	}
	return resps, nil
}

func (s *responseStore) DeleteByRequestId(ctx context.Context, id string) error {
	s.m.Lock()
	defer s.m.Unlock()
	delete(s.data, id)
	_, err := s.database.Collection(ResponseCollection).DeleteMany(ctx, bson.M{"request_id": id})
	return err
}

func (s *responseStore) DeleteByResponseId(ctx context.Context, id string) error {
	s.m.Lock()
	defer s.m.Unlock()
	for reqID, responses := range s.data {
		var updatedResponses []*messages.CrackHashWorkerResponse
		for _, r := range responses {
			if r.Id != id {
				updatedResponses = append(updatedResponses, r)
			}
		}
		s.data[reqID] = updatedResponses
	}
	result, err := s.database.Collection(ResponseCollection).DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return errors.Wrap(err, "failed to delete response by id")
	}
	if result.DeletedCount == 0 {
		return NotFoundErr
	}
	return nil
}

func (s *responseStore) loadStore(ctx context.Context, id string) error {
	cursor, err := s.database.Collection(ResponseCollection).Find(ctx, bson.M{"requestId": id})
	if err != nil {
		return err
	}
	defer func() { _ = cursor.Close(ctx) }()
	var resps []*messages.CrackHashWorkerResponse
	for cursor.Next(ctx) {
		var resp *messages.CrackHashWorkerResponse
		if err := cursor.Decode(resp); err != nil {
			return err
		}
		resps = append(resps, resp)
	}
	s.data[id] = resps
	return nil
}
