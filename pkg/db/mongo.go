package db

import (
	"context"
	"time"

	"github.com/invenlore/core/pkg/config"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

func MongoDBConnect(ctx context.Context, mongoCfg *config.MongoConfig, opts ...*options.ClientOptions) (*mongo.Client, error) {
	uri := mongoCfg.URI

	clientOpts := options.MergeClientOptions(append([]*options.ClientOptions{options.Client().ApplyURI(uri)}, opts...)...)

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, err
	}

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		_ = client.Disconnect(context.Background())

		return nil, err
	}

	return client, nil
}
