package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/araihu/balemoh/plugin-sdk/notifications"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNotificationCapacityFailureRoundTripsAsNonRetryable(t *testing.T) {
	rpcErr := toNotificationRPC(notifications.Fail("capacity_exceeded"))
	if status.Code(rpcErr) != codes.ResourceExhausted {
		t.Fatalf("status code = %s, want ResourceExhausted", status.Code(rpcErr))
	}
	statusValue, ok := status.FromError(rpcErr)
	if !ok {
		t.Fatal("capacity error is not a gRPC status")
	}
	var info *errdetails.ErrorInfo
	for _, detail := range statusValue.Details() {
		if candidate, ok := detail.(*errdetails.ErrorInfo); ok {
			info = candidate
			break
		}
	}
	if info == nil || info.Domain != notifications.ErrorInfoDomain || info.Reason != notifications.ErrorInfoReasonCapacity {
		t.Fatalf("capacity detail = %+v", info)
	}
	var result *notifications.Error
	if !errors.As(fromNotificationRPC(rpcErr), &result) || result.Code != "capacity_exceeded" || result.Retryable {
		t.Fatalf("round-tripped capacity error = %+v", result)
	}
	result = nil
	if !errors.As(fromNotificationRPC(toNotificationRPC(notifications.Fail("resource_exhausted"))), &result) || result.Code != "resource_exhausted" || !result.Retryable {
		t.Fatalf("round-tripped busy error = %+v", result)
	}
	result = nil
	if !errors.As(fromNotificationRPC(status.Error(codes.ResourceExhausted, "inbox capacity is full")), &result) || result.Code != "resource_exhausted" || !result.Retryable {
		t.Fatalf("human-text capacity error was reclassified = %+v", result)
	}
}

func TestNotificationUnknownFailureIsSanitizedAcrossRPCMapping(t *testing.T) {
	rpcErr := toNotificationRPC(errors.New("database password=secret"))
	if status.Code(rpcErr) != codes.Unavailable || strings.Contains(rpcErr.Error(), "database password=secret") || strings.Contains(rpcErr.Error(), "secret") {
		t.Fatalf("unsafe unknown status: %v", rpcErr)
	}
	var result *notifications.Error
	if !errors.As(fromNotificationRPC(rpcErr), &result) || result.Code != "unavailable" || result.Retryable == false {
		t.Fatalf("round-tripped unknown error = %+v", result)
	}
}
