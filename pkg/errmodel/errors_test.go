package errmodel

import (
	"context"
	"testing"

	"github.com/invenlore/core/pkg/logger"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestErrorAddsRequestID(t *testing.T) {
	ctx := context.WithValue(context.Background(), logger.RequestIDCtxKey, "req-1")

	err := Error(ctx, codes.Internal, "boom")
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected grpc status error")
	}

	var found bool
	for _, detail := range st.Details() {
		if info, ok := detail.(*errdetails.RequestInfo); ok {
			found = true
			if info.RequestId != "req-1" {
				t.Fatalf("expected request_id req-1, got %s", info.RequestId)
			}
		}
	}

	if !found {
		t.Fatalf("expected RequestInfo detail")
	}
}

func TestBadRequestAddsViolations(t *testing.T) {
	err := BadRequest(context.Background(), "invalid", FieldViolation("email", "required"))
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected grpc status error")
	}

	var badRequest *errdetails.BadRequest
	for _, detail := range st.Details() {
		if br, ok := detail.(*errdetails.BadRequest); ok {
			badRequest = br
		}
	}

	if badRequest == nil || len(badRequest.FieldViolations) == 0 {
		t.Fatalf("expected bad request detail with field violations")
	}
}

func TestErrorInfoAddsDomain(t *testing.T) {
	err := ErrorInfo(context.Background(), codes.PermissionDenied, "forbidden", "AUTHZ_DENIED", map[string]string{"scope": "admin"})
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected grpc status error")
	}

	var info *errdetails.ErrorInfo
	for _, detail := range st.Details() {
		if ei, ok := detail.(*errdetails.ErrorInfo); ok {
			info = ei
		}
	}

	if info == nil {
		t.Fatalf("expected ErrorInfo detail")
	}

	if info.Domain != DefaultErrorDomain {
		t.Fatalf("expected domain %s, got %s", DefaultErrorDomain, info.Domain)
	}
}
