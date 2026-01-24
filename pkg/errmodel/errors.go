package errmodel

import (
	"context"
	"time"

	"github.com/invenlore/core/pkg/logger"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
)

const DefaultErrorDomain = "invenlore"

func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	if requestID, ok := ctx.Value(logger.RequestIDCtxKey).(string); ok && requestID != "" {
		return requestID
	}

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get(logger.RequestIDMDKey); len(vals) > 0 && vals[0] != "" {
			return vals[0]
		}
	}

	return ""
}

func NewStatus(ctx context.Context, code codes.Code, message string, details ...proto.Message) *status.Status {
	statusProto := &statuspb.Status{
		Code:    int32(code),
		Message: message,
		Details: make([]*anypb.Any, 0, len(details)+1),
	}

	for _, detail := range details {
		if detail == nil {
			continue
		}

		if packed, err := anypb.New(detail); err == nil {
			statusProto.Details = append(statusProto.Details, packed)
		}
	}

	if requestID := RequestIDFromContext(ctx); requestID != "" {
		if packed, err := anypb.New(&errdetails.RequestInfo{RequestId: requestID}); err == nil {
			statusProto.Details = append(statusProto.Details, packed)
		}
	}

	return status.FromProto(statusProto)
}

func Error(ctx context.Context, code codes.Code, message string, details ...proto.Message) error {
	return NewStatus(ctx, code, message, details...).Err()
}

func BadRequest(ctx context.Context, message string, violations ...*errdetails.BadRequest_FieldViolation) error {
	if len(violations) == 0 {
		return Error(ctx, codes.InvalidArgument, message)
	}

	return Error(ctx, codes.InvalidArgument, message, &errdetails.BadRequest{FieldViolations: violations})
}

func ErrorInfo(ctx context.Context, code codes.Code, message, reason string, metadata map[string]string) error {
	return Error(ctx, code, message, &errdetails.ErrorInfo{
		Reason:   reason,
		Domain:   DefaultErrorDomain,
		Metadata: metadata,
	})
}

func Retryable(ctx context.Context, code codes.Code, message string, delay time.Duration) error {
	return Error(ctx, code, message, &errdetails.RetryInfo{RetryDelay: durationpb.New(delay)})
}

func FieldViolation(field, description string) *errdetails.BadRequest_FieldViolation {
	return &errdetails.BadRequest_FieldViolation{
		Field:       field,
		Description: description,
	}
}
