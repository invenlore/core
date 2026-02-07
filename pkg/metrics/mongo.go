package metrics

import (
	"context"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/event"
)

type MongoMetrics struct {
	operations *OperationMetrics

	mu        sync.Mutex
	entries   map[int64]mongoCommandEntry
	maxAge    time.Duration
	maxStored int
}

type mongoCommandEntry struct {
	collection string
	startedAt  time.Time
}

func NewMongoMetrics(reg *Registry) *MongoMetrics {
	if reg == nil {
		return nil
	}

	return &MongoMetrics{
		operations: NewOperationMetrics(reg),
		entries:    make(map[int64]mongoCommandEntry),
		maxAge:     5 * time.Minute,
		maxStored:  1024,
	}
}

func (m *MongoMetrics) Monitor() *event.CommandMonitor {
	if m == nil || m.operations == nil {
		return nil
	}

	return &event.CommandMonitor{
		Started:   m.started,
		Succeeded: m.succeeded,
		Failed:    m.failed,
	}
}

func (m *MongoMetrics) started(_ context.Context, ev *event.CommandStartedEvent) {
	if ev == nil {
		return
	}

	collection := extractCollection(ev.CommandName, ev.Command)
	if collection == "" {
		collection = "unknown"
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.entries) >= m.maxStored {
		m.cleanupLocked(time.Now())
	}

	m.entries[ev.RequestID] = mongoCommandEntry{
		collection: collection,
		startedAt:  time.Now(),
	}
}

func (m *MongoMetrics) succeeded(_ context.Context, event *event.CommandSucceededEvent) {
	if event == nil {
		return
	}

	operation := event.CommandName
	if operation == "" {
		operation = "unknown"
	}

	duration := event.Duration
	if duration == 0 {
		duration = time.Duration(event.DurationNanos)
	}

	collection := m.lookupCollection(event.RequestID, event.DatabaseName, event.CommandName)
	m.operations.ObserveMongo(operation, collection, "ok", duration)
}

func (m *MongoMetrics) failed(_ context.Context, event *event.CommandFailedEvent) {
	if event == nil {
		return
	}

	operation := event.CommandName
	if operation == "" {
		operation = "unknown"
	}

	duration := event.Duration
	if duration == 0 {
		duration = time.Duration(event.DurationNanos)
	}

	collection := m.lookupCollection(event.RequestID, event.DatabaseName, event.CommandName)
	m.operations.ObserveMongo(operation, collection, "error", duration)
}

func (m *MongoMetrics) lookupCollection(requestID int64, database, commandName string) string {
	m.mu.Lock()
	entry, ok := m.entries[requestID]
	if ok {
		delete(m.entries, requestID)
	}
	m.cleanupLocked(time.Now())
	m.mu.Unlock()

	if ok && entry.collection != "" {
		return entry.collection
	}

	if database != "" {
		return database
	}

	if commandName != "" {
		return commandName
	}

	return "unknown"
}

func (m *MongoMetrics) cleanupLocked(now time.Time) {
	if m.maxAge <= 0 {
		return
	}

	if len(m.entries) > m.maxStored && m.maxStored > 0 {
		m.trimLocked(len(m.entries) - m.maxStored)
	}

	for key, entry := range m.entries {
		if now.Sub(entry.startedAt) > m.maxAge {
			delete(m.entries, key)
		}
	}
}

func (m *MongoMetrics) trimLocked(overflow int) {
	if overflow <= 0 {
		return
	}

	for key := range m.entries {
		delete(m.entries, key)

		overflow--
		if overflow <= 0 {
			return
		}
	}
}

func extractCollection(commandName string, command bson.Raw) string {
	if commandName == "" || command == nil {
		return ""
	}

	if val := command.Lookup(commandName); val.Type != bson.TypeNull && val.Type != bson.TypeUndefined {
		if name, ok := val.StringValueOK(); ok {
			return name
		}
	}

	return ""
}
