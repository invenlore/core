package logger

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type wrappedServerStreamLogger struct {
	stream      grpc.ServerStream
	reqID       string
	modifiedCtx context.Context
}

func (w *wrappedServerStreamLogger) SendMsg(m any) error {
	err := w.stream.SendMsg(m)

	if err != nil {
		statusCode := status.Code(err)

		logrus.WithFields(logrus.Fields{
			"scope":      "gRPC",
			"requestID":  w.reqID,
			"statusCode": statusCode,
			"error":      err.Error(),
		}).Errorf("server: failed to send message in gRPC stream")
	} else {
		logrus.WithFields(logrus.Fields{
			"scope":     "gRPC",
			"requestID": w.reqID,
		}).Trace("server: sent message in gRPC stream")
	}

	return err
}

func (w *wrappedServerStreamLogger) RecvMsg(m any) error {
	err := w.stream.RecvMsg(m)

	if err != nil {
		statusCode := status.Code(err)

		logrus.WithFields(logrus.Fields{
			"scope":      "gRPC",
			"requestID":  w.reqID,
			"statusCode": statusCode,
			"error":      err.Error(),
		}).Errorf("server: failed to receive message in gRPC stream")
	} else {
		logrus.WithFields(logrus.Fields{
			"scope":     "gRPC",
			"requestID": w.reqID,
		}).Trace("server: received message in gRPC stream")
	}

	return err
}

func (w *wrappedServerStreamLogger) SetHeader(md metadata.MD) error {
	return w.stream.SetHeader(md)
}

func (w *wrappedServerStreamLogger) SendHeader(md metadata.MD) error {
	return w.stream.SendHeader(md)
}

func (w *wrappedServerStreamLogger) SetTrailer(md metadata.MD) {
	w.stream.SetTrailer(md)
}

func (w *wrappedServerStreamLogger) Context() context.Context {
	return w.modifiedCtx
}

func ServerRequestIDInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	requestID := ""

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get(RequestIDMDKey); len(vals) > 0 && vals[0] != "" {
			requestID = vals[0]
		}
	}

	if requestID == "" {
		requestID = uuid.NewString()
	}

	newCtx := context.WithValue(ctx, RequestIDCtxKey, requestID)
	return handler(newCtx, req)
}

func ServerLoggingInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	reqID, _ := ctx.Value(RequestIDCtxKey).(string)
	if reqID == "" {
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if vals := md.Get(RequestIDMDKey); len(vals) > 0 && vals[0] != "" {
				reqID = vals[0]
			}
		}
	}

	if reqID == "" {
		reqID = "no-request-id"
	}

	startTime := time.Now()

	logrus.WithFields(logrus.Fields{
		"scope":     "gRPC",
		"requestID": reqID,
	}).Tracef("server: received gRPC request: %s", info.FullMethod)

	resp, err := handler(ctx, req)

	duration := time.Since(startTime)
	statusCode := status.Code(err)

	logFields := logrus.Fields{
		"scope":      "gRPC",
		"requestID":  reqID,
		"took":       duration,
		"method":     info.FullMethod,
		"statusCode": statusCode,
	}

	if err != nil {
		logFields["error"] = err.Error()
		logrus.WithFields(logFields).Errorf("server: gRPC request failed")
	} else {
		logrus.WithFields(logFields).Trace("server: gRPC request completed successfully")
	}

	return resp, err
}

func ServerStreamRequestIDInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := ss.Context()
	requestID := ""

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get(RequestIDMDKey); len(vals) > 0 && vals[0] != "" {
			requestID = vals[0]
		}
	}

	if requestID == "" {
		requestID = uuid.NewString()
	}

	newCtx := context.WithValue(ctx, RequestIDCtxKey, requestID)

	wrappedStreamWithNewCtx := &wrappedServerStreamLogger{
		stream:      ss,
		reqID:       requestID,
		modifiedCtx: newCtx,
	}

	return handler(srv, wrappedStreamWithNewCtx)
}

func ServerStreamLoggingInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	reqIDStr := "no-request-id"

	if ws, ok := ss.(*wrappedServerStreamLogger); ok {
		reqIDStr = ws.reqID
	} else {
		if v, ok := ss.Context().Value(RequestIDCtxKey).(string); ok && v != "" {
			reqIDStr = v
		} else if md, ok := metadata.FromIncomingContext(ss.Context()); ok {
			if vals := md.Get(RequestIDMDKey); len(vals) > 0 && vals[0] != "" {
				reqIDStr = vals[0]
			}
		}
	}

	startTime := time.Now()

	logrus.WithFields(logrus.Fields{
		"scope":     "gRPC",
		"requestID": reqIDStr,
	}).Tracef("server: received gRPC stream: %s", info.FullMethod)

	err := handler(srv, ss)

	duration := time.Since(startTime)
	statusCode := status.Code(err)

	logFields := logrus.Fields{
		"scope":      "gRPC",
		"requestID":  reqIDStr,
		"took":       duration,
		"method":     info.FullMethod,
		"statusCode": statusCode,
	}

	if err != nil {
		logFields["error"] = err.Error()
		logrus.WithFields(logFields).Errorf("server: gRPC stream failed")
	} else {
		logrus.WithFields(logFields).Trace("server: gRPC stream completed successfully")
	}

	return err
}
