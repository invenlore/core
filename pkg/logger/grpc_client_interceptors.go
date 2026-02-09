package logger

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type wrappedClientStreamLogger struct {
	grpc.ClientStream
	ctx       context.Context
	reqID     string
	method    string
	target    string
	startTime time.Time
}

func (w *wrappedClientStreamLogger) Context() context.Context {
	return w.ctx
}

func (w *wrappedClientStreamLogger) SendMsg(m any) error {
	err := w.ClientStream.SendMsg(m)

	if err != nil {
		duration := time.Since(w.startTime)
		statusCode := status.Code(err)

		logrus.WithFields(logrus.Fields{
			"scope":      "gRPC",
			"request_id": w.reqID,
			"latency_ms": duration.Milliseconds(),
			"rpc_method": w.method,
			"target":     w.target,
			"grpc_code":  statusCode,
			"error":      err.Error(),
		}).Errorf("client: failed to send message in stream")
	} else {
		logrus.WithFields(logrus.Fields{
			"scope":      "gRPC",
			"request_id": w.reqID,
		}).Tracef("client: sent message in stream %s", w.method)
	}

	return err
}

func (w *wrappedClientStreamLogger) RecvMsg(m any) error {
	err := w.ClientStream.RecvMsg(m)

	if err != nil {
		if err != io.EOF {
			duration := time.Since(w.startTime)
			statusCode := status.Code(err)

			logrus.WithFields(logrus.Fields{
				"scope":      "gRPC",
				"request_id": w.reqID,
				"latency_ms": duration.Milliseconds(),
				"rpc_method": w.method,
				"target":     w.target,
				"grpc_code":  statusCode,
				"error":      err.Error(),
			}).Errorf("client: failed to receive message in stream")
		}
	} else {
		logrus.WithFields(logrus.Fields{
			"scope":      "gRPC",
			"request_id": w.reqID,
		}).Tracef("client: received message in stream %s", w.method)
	}

	return err
}

func ClientRequestIDInterceptor(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	requestID, _ := ctx.Value(RequestIDCtxKey).(string)
	if requestID == "" {
		if md, ok := metadata.FromOutgoingContext(ctx); ok {
			if vals := md.Get(RequestIDMDKey); len(vals) > 0 && vals[0] != "" {
				requestID = vals[0]
			}
		}
	}

	if requestID == "" {
		requestID = uuid.NewString()
	}

	ctx = context.WithValue(ctx, RequestIDCtxKey, requestID)

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	} else {
		md = md.Copy()
	}

	md.Set(RequestIDMDKey, requestID)
	newCtx := metadata.NewOutgoingContext(ctx, md)

	logrus.WithFields(logrus.Fields{
		"scope":      "gRPC",
		"request_id": requestID,
	}).Tracef(
		"client: outgoing request to %s, method: %s",
		cc.Target(),
		method,
	)

	return invoker(newCtx, method, req, reply, cc, opts...)
}

func ClientLoggingInterceptor(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	requestID, _ := ctx.Value(RequestIDCtxKey).(string)
	if requestID == "" {
		if md, ok := metadata.FromOutgoingContext(ctx); ok {
			if vals := md.Get(RequestIDMDKey); len(vals) > 0 && vals[0] != "" {
				requestID = vals[0]
			}
		}
	}

	if requestID == "" {
		requestID = "no-request-id"
	}

	startTime := time.Now()

	logrus.WithFields(logrus.Fields{
		"scope":      "gRPC",
		"request_id": requestID,
	}).Tracef(
		"client: sending gRPC request: %s (target: %s)",
		method,
		cc.Target(),
	)

	err := invoker(ctx, method, req, reply, cc, opts...)

	duration := time.Since(startTime)
	statusCode := status.Code(err)

	logFields := logrus.Fields{
		"scope":      "gRPC",
		"request_id": requestID,
		"latency_ms": duration.Milliseconds(),
		"rpc_method": method,
		"grpc_code":  statusCode,
		"target":     cc.Target(),
	}

	if err != nil {
		logFields["error"] = err.Error()
		logrus.WithFields(logFields).Errorf("client: gRPC request failed")
	} else {
		logrus.WithFields(logFields).Tracef("client: gRPC request completed successfully")
	}

	return err
}

func ClientStreamInterceptor(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	requestID, _ := ctx.Value(RequestIDCtxKey).(string)
	if requestID == "" {
		if md, ok := metadata.FromOutgoingContext(ctx); ok {
			if vals := md.Get(RequestIDMDKey); len(vals) > 0 && vals[0] != "" {
				requestID = vals[0]
			}
		}
	}

	if requestID == "" {
		requestID = uuid.NewString()
	}

	ctx = context.WithValue(ctx, RequestIDCtxKey, requestID)

	logFields := logrus.Fields{
		"scope":      "gRPC",
		"request_id": requestID,
	}

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	} else {
		md = md.Copy()
	}

	md.Set(RequestIDMDKey, requestID)
	newCtx := metadata.NewOutgoingContext(ctx, md)

	startTime := time.Now()
	logrus.WithFields(logFields).Tracef("client: initiating stream: %s (target: %s)", method, cc.Target())

	actualClientStream, err := streamer(newCtx, desc, cc, method, opts...)
	if err != nil {
		logrus.WithFields(logFields).Errorf("client: failed to call streamer function: %v", err)

		return nil, err
	}

	if actualClientStream == nil {
		logrus.WithFields(logFields).Errorf("client: streamer function returned nil ClientStream")

		return nil, err
	}

	if _, ok := actualClientStream.(*wrappedClientStreamLogger); ok {
		return actualClientStream, nil
	}

	wrapped := &wrappedClientStreamLogger{
		ClientStream: actualClientStream,
		ctx:          newCtx,
		reqID:        requestID,
		method:       method,
		target:       cc.Target(),
		startTime:    startTime,
	}

	return wrapped, nil
}
