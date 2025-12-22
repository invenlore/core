package db

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func MongoGateUnary(m *MongoReadiness) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if strings.HasSuffix(info.FullMethod, "/HealthCheck") {
			return handler(ctx, req)
		}

		if !m.Ready() {
			msg := m.LastError()
			if msg == "" {
				msg = "MongoDB unavailable"
			}

			return nil, status.Error(codes.Unavailable, msg)
		}

		return handler(ctx, req)
	}
}

func MongoGateStream(m *MongoReadiness) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if strings.HasSuffix(info.FullMethod, "/HealthCheck") {
			return handler(srv, ss)
		}

		if !m.Ready() {
			msg := m.LastError()
			if msg == "" {
				msg = "MongoDB unavailable"
			}

			return status.Error(codes.Unavailable, msg)
		}

		return handler(srv, ss)
	}
}
