package runtime

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/araihu/balemoh/plugin-sdk/notifications"
	pb "github.com/araihu/balemoh/plugin-sdk/protocol/v1alpha1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func TestNotificationEscapingHeavyMessageUsesCanonicalJSONLimit(t *testing.T) {
	message := notifications.Message{
		EventID:    "event-escaping",
		Severity:   notifications.SeverityWarning,
		Title:      "Escaping-heavy body",
		Body:       strings.Repeat("\u2028", 3000),
		OccurredAt: time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC),
	}
	encoded, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) <= notifications.MaxEnvelope {
		t.Fatalf("test message JSON size = %d, want > %d", len(encoded), notifications.MaxEnvelope)
	}
	wire := &pb.NotificationPublishRequest{Message: &pb.NotificationMessage{
		EventId: message.EventID, Severity: message.Severity, Title: message.Title,
		Body: message.Body, OccurredAt: message.OccurredAt.UTC().Format(time.RFC3339Nano),
	}}
	if size := proto.Size(wire); size >= notifications.MaxEnvelope {
		t.Fatalf("test message protobuf size = %d, want < %d", size, notifications.MaxEnvelope)
	}
	if _, err := notificationRequest(message); notifications.Code(err) != "invalid_argument" {
		t.Fatalf("escaping-heavy message accepted: %v", err)
	}
}

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
