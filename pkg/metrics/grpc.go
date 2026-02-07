package metrics

import (
	"context"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

type GRPCServerMetrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func NewGRPCServerMetrics(reg *Registry) *GRPCServerMetrics {
	if reg == nil {
		return nil
	}

	requests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grpc_server_requests_total",
			Help: "Total number of gRPC server requests.",
		},
		[]string{"method", "code"},
	)

	duration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "grpc_server_handling_seconds",
			Help:    "Histogram of gRPC server request durations in seconds.",
			Buckets: DefaultBuckets,
		},
		[]string{"method", "code"},
	)

	reg.Registerer.MustRegister(requests, duration)

	return &GRPCServerMetrics{
		requests: requests,
		duration: duration,
	}
}

func (m *GRPCServerMetrics) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	if m == nil {
		return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(ctx, req)
		}
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)

		m.observe(info.FullMethod, err, time.Since(start))
		return resp, err
	}
}

func (m *GRPCServerMetrics) StreamServerInterceptor() grpc.StreamServerInterceptor {
	if m == nil {
		return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			return handler(srv, ss)
		}
	}

	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		err := handler(srv, ss)

		m.observe(info.FullMethod, err, time.Since(start))
		return err
	}
}

func (m *GRPCServerMetrics) observe(fullMethod string, err error, took time.Duration) {
	method := NormalizeGRPCMethod(fullMethod)
	code := status.Code(err).String()

	if m.requests != nil {
		m.requests.WithLabelValues(method, code).Inc()
	}

	if m.duration != nil {
		m.duration.WithLabelValues(method, code).Observe(took.Seconds())
	}
}

func NormalizeGRPCMethod(fullMethod string) string {
	if fullMethod == "" {
		return "unknown"
	}

	trimmed := strings.TrimPrefix(fullMethod, "/")
	if trimmed == "" {
		return "unknown"
	}

	return trimmed
}
