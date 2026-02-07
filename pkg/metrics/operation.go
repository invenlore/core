package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type OperationMetrics struct {
	mongoDuration   *prometheus.HistogramVec
	mongoOperations *prometheus.CounterVec
}

func NewOperationMetrics(reg *Registry) *OperationMetrics {
	if reg == nil {
		return nil
	}

	mongoDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "invenlore_mongo_operation_seconds",
			Help:    "MongoDB operation duration in seconds.",
			Buckets: DefaultBuckets,
		},
		[]string{"operation", "collection", "result"},
	)

	mongoOperations := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "invenlore_mongo_operations_total",
			Help: "MongoDB operations total.",
		},
		[]string{"operation", "collection", "result"},
	)

	reg.Registerer.MustRegister(mongoDuration, mongoOperations)

	return &OperationMetrics{
		mongoDuration:   mongoDuration,
		mongoOperations: mongoOperations,
	}
}

func (m *OperationMetrics) ObserveMongo(operation, collection, result string, duration time.Duration) {
	if m == nil {
		return
	}

	if operation == "" {
		operation = "unknown"
	}
	if collection == "" {
		collection = "unknown"
	}
	if result == "" {
		result = "error"
	}

	m.mongoOperations.WithLabelValues(operation, collection, result).Inc()
	m.mongoDuration.WithLabelValues(operation, collection, result).Observe(duration.Seconds())
}
