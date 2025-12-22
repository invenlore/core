package db

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type MongoReadiness struct {
	client  *mongo.Client
	timeout time.Duration

	ready   atomic.Bool
	lastErr atomic.Value
}

func NewMongoReadiness(client *mongo.Client, timeout time.Duration) *MongoReadiness {
	m := &MongoReadiness{client: client, timeout: timeout}

	m.ready.Store(false)
	m.lastErr.Store("")

	return m
}

func (m *MongoReadiness) Ready() bool {
	return m.ready.Load()
}

func (m *MongoReadiness) LastError() string {
	s, _ := m.lastErr.Load().(string)

	return s
}

func (m *MongoReadiness) CheckNow(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	err := m.client.Ping(pingCtx, readpref.Primary())

	if err == nil {
		m.lastErr.Store("")

		prev := m.ready.Swap(true)
		if !prev {
			logrus.WithField("scope", "health").Infof("MongoDB is available")
		}

		return nil
	}

	m.lastErr.Store(err.Error())

	prev := m.ready.Swap(false)
	if prev {
		logrus.WithField("scope", "health").Warnf("MongoDB is unavailable: %v", err)
	}

	return err
}

func (m *MongoReadiness) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()

	_ = m.CheckNow(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = m.CheckNow(ctx)
		}
	}
}
