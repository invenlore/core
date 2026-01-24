package db

import (
	"context"

	"github.com/invenlore/core/pkg/errmodel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

func MongoGateUnary(m *MongoReadiness, allowMethods ...string) grpc.UnaryServerInterceptor {
	allow := make(map[string]struct{}, len(allowMethods))
	for _, s := range allowMethods {
		allow[s] = struct{}{}
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if _, ok := allow[info.FullMethod]; ok {
			return handler(ctx, req)
		}

		if !m.Ready() {
			return nil, errmodel.Error(ctx, codes.Unavailable, m.LastError())
		}

		return handler(ctx, req)
	}
}

func MongoGateStream(m *MongoReadiness, allowMethods ...string) grpc.StreamServerInterceptor {
	allow := make(map[string]struct{}, len(allowMethods))
	for _, s := range allowMethods {
		allow[s] = struct{}{}
	}

	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if _, ok := allow[info.FullMethod]; ok {
			return handler(srv, ss)
		}

		if !m.Ready() {
			return errmodel.Error(ss.Context(), codes.Unavailable, m.LastError())
		}

		return handler(srv, ss)
	}
}
