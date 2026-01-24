package recovery

import (
	"context"

	"github.com/invenlore/core/pkg/errmodel"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

func RecoveryUnaryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("panic in unary handler %s: %v", info.FullMethod, r)
			err = errmodel.Error(ctx, codes.Internal, "internal error")
		}
	}()

	return handler(ctx, req)
}

func RecoveryStreamInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("panic in stream handler %s: %v", info.FullMethod, r)
			err = errmodel.Error(ss.Context(), codes.Internal, "internal error")
		}
	}()

	return handler(srv, ss)
}
