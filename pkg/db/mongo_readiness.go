package db

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

const (
	mongoNotCheckedYet string = "MongoDB not checked yet"
	mongoGateClosed    string = "MongoDB gate closed"
	mongoUnavailable   string = "MongoDB unavailable"
	mongoIsDown        string = "MongoDB is down"
	mongoIsUp          string = "MongoDB is up"
)

type MongoReadiness struct {
	client  *mongo.Client
	timeout time.Duration

	mongoUp  atomic.Bool
	mongoErr atomic.Value // string

	gateOpen   atomic.Bool
	gateReason atomic.Value // string
}

func NewMongoReadiness(client *mongo.Client, timeout time.Duration) *MongoReadiness {
	m := &MongoReadiness{
		client:  client,
		timeout: timeout,
	}

	m.mongoUp.Store(false)
	m.mongoErr.Store(mongoNotCheckedYet)

	m.gateOpen.Store(true)
	m.gateReason.Store("")

	return m
}

func (m *MongoReadiness) CloseGate(reason string) {
	if reason == "" {
		reason = mongoGateClosed
	}

	m.gateReason.Store(reason)
	m.gateOpen.Store(false)
}

func (m *MongoReadiness) OpenGate() {
	m.gateReason.Store("")
	m.gateOpen.Store(true)
}

func (m *MongoReadiness) Ready() bool {
	return m.gateOpen.Load() && m.mongoUp.Load()
}

func (m *MongoReadiness) LastError() string {
	if !m.gateOpen.Load() {
		if s, _ := m.gateReason.Load().(string); s != "" {
			return s
		}

		return mongoGateClosed
	}

	if !m.mongoUp.Load() {
		if s, _ := m.mongoErr.Load().(string); s != "" {
			return s
		}

		return mongoUnavailable
	}

	return ""
}

func (m *MongoReadiness) CheckNow(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	err := m.client.Ping(ctx, readpref.Primary())
	if err != nil {
		m.mongoErr.Store(err.Error())

		// MongoDB down
		prev := m.mongoUp.Swap(false)
		if prev {
			logrus.WithField("scope", "health").WithError(err).Warn(mongoIsDown)
		}

		return err
	}

	m.mongoErr.Store("")

	// MongoDB up
	prev := m.mongoUp.Swap(true)
	if !prev {
		logrus.WithField("scope", "health").Info(mongoIsUp)
	}

	return nil
}

func (m *MongoReadiness) Run(ctx context.Context, interval time.Duration) {
	_ = m.CheckNow(ctx)

	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = m.CheckNow(ctx)
		}
	}
}
