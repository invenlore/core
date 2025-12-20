package db

import (
	"context"

	"github.com/invenlore/core/pkg/config"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func MongoDBConnect(cfg *config.MongoConfig) *mongo.Client {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.OperationTimeout)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.URI))
	if err != nil {
		logrus.Fatalf("couldn't connect to MongoDB: %v", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		logrus.Fatalf("MongoDB isn't available: %v", err)
	}

	logrus.Debug("MongoDB connected successfully")

	return client
}
