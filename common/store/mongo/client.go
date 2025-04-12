package mongo

import (
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func NewClient(cfg *ClientConfig) (*mongo.Client, error) {
	creds := options.Credential{
		Username: cfg.Username,
		Password: cfg.Password,
	}
	logOpts := options.
		Logger().
		SetSink(&logger{}).
		SetComponentLevel(options.LogComponentCommand, options.LogLevelDebug).
		SetComponentLevel(options.LogComponentConnection, options.LogLevelDebug)
	bsonOpts := &options.BSONOptions{
		UseJSONStructTags: true,
		NilSliceAsEmpty:   true,
	}
	opts := options.
		Client().
		ApplyURI(cfg.URI).
		SetAuth(creds).
		SetLoggerOptions(logOpts).
		SetBSONOptions(bsonOpts)
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to MongoDB")
	}
	return client, nil
}
