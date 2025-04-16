package requeststore

import (
	"context"
	"github.com/pkg/errors"
	"github.com/ykhdr/crack-hash/manager/internal/messages/request"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"sync"
)

const (
	RequestCollection = "requests"
)

var NotFoundErr = errors.New("not found")

type RequestStore interface {
	Get(ctx context.Context, id request.Id) (*request.Info, error)
	List(ctx context.Context) ([]*request.Info, error)
	Save(ctx context.Context, req *request.Info) error
	Delete(ctx context.Context, id request.Id) error
	DeleteFromCache(id request.Id)
	UpdateStatus(ctx context.Context, id request.Id, status request.Status, errorReason string) error
	Update(ctx context.Context, req *request.Info) error
}

type requestStore struct {
	data     map[request.Id]*request.Info
	database *mongo.Database
	m        sync.RWMutex
}

func NewRequestStore(database *mongo.Database) RequestStore {
	return &requestStore{
		data:     make(map[request.Id]*request.Info),
		database: database,
	}
}

func (s *requestStore) Get(ctx context.Context, id request.Id) (*request.Info, error) {
	s.m.RLock()
	defer s.m.RUnlock()
	req, exists := s.data[id]
	if !exists {
		var r request.Info
		filter := bson.M{"_id": id}
		if err := s.database.Collection(RequestCollection).FindOne(ctx, filter).Decode(&r); err != nil {
			return nil, NotFoundErr
		}
		req = &r
		s.data[id] = req
	}
	return req.Copy(), nil
}

func (s *requestStore) Save(ctx context.Context, req *request.Info) error {
	s.m.Lock()
	defer s.m.Unlock()
	reqCopy := req.Copy()
	s.data[req.ID] = req.Copy()
	_, err := s.database.Collection(RequestCollection).InsertOne(ctx, reqCopy)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return s.Update(ctx, req)
		}
		return errors.Wrap(err, "error saving request")
	}
	return nil
}

func (s *requestStore) Delete(ctx context.Context, id request.Id) error {
	s.m.Lock()
	defer s.m.Unlock()
	delete(s.data, id)
	_, err := s.database.Collection(RequestCollection).DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return errors.Wrap(err, "error deleting request")
	}
	return nil
}

func (s *requestStore) DeleteFromCache(id request.Id) {
	s.m.Lock()
	defer s.m.Unlock()
	delete(s.data, id)
}

func (s *requestStore) UpdateStatus(ctx context.Context, id request.Id, status request.Status, errorReason string) error {
	s.m.Lock()
	defer s.m.Unlock()
	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": bson.M{
			"status":       status,
			"error_reason": errorReason,
		},
	}
	result, err := s.database.Collection(RequestCollection).UpdateOne(ctx, filter, update)
	if err != nil {
		return errors.Wrap(err, "error updating request status")
	}
	if result.MatchedCount == 0 {
		return NotFoundErr
	}
	if req, exists := s.data[id]; exists {
		req.Status = status
		req.ErrorReason = errorReason
	}
	return nil
}

func (s *requestStore) List(ctx context.Context) ([]*request.Info, error) {
	s.m.RLock()
	defer s.m.RUnlock()
	cursor, err := s.database.Collection(RequestCollection).Find(ctx, bson.M{})
	if err != nil {
		return nil, errors.Wrap(err, "error listing requests")
	}
	defer func() { _ = cursor.Close(ctx) }()
	var result []*request.Info
	for cursor.Next(ctx) {
		var req request.Info
		if err := cursor.Decode(&req); err != nil {
			return nil, errors.Wrap(err, "error listing requests")
		}
		result = append(result, &req)
		s.data[req.ID] = &req
	}
	return result, nil
}

func (s *requestStore) Update(ctx context.Context, info *request.Info) error {
	s.m.RLock()
	defer s.m.RUnlock()
	filter := bson.M{"_id": info.ID}
	update := bson.M{
		"$set": bson.M{
			"status":              info.Status,
			"request":             info.Request,
			"found_data":          info.FoundData,
			"created_at":          info.CreatedAt,
			"error_reason":        info.ErrorReason,
			"ready_service_count": info.ReadyServiceCount,
		},
	}
	result, err := s.database.Collection("requests").UpdateOne(ctx, filter, update)
	if err != nil {
		return errors.Wrap(err, "failed to update info document")
	}
	if result.MatchedCount == 0 {
		return NotFoundErr
	}
	s.data[info.ID] = info.Copy()
	return nil
}
