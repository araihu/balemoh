package federation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/araihu/balemoh/internal/application/catalog"
)

func TestPublisherPostsSnapshotWithoutPinState(t *testing.T) {
	const token = "agent-secret"
	var receivedJSON string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/federation/snapshots" {
			t.Fatalf("path = %q, want federation snapshot path", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Fatalf("authorization = %q, want bearer token", got)
		}
		var payload map[string]any
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&payload); err != nil {
			t.Fatalf("decode raw payload: %v", err)
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal raw payload: %v", err)
		}
		receivedJSON = string(encoded)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	publisher, err := NewPublisher(server.URL, token)
	if err != nil {
		t.Fatalf("NewPublisher() error = %v", err)
	}
	candidate := catalog.NewCandidate(
		catalog.SourceRef{Kind: "container", ID: "docker-local"},
		catalog.ResourceRef{Kind: "container", Name: "whoami"},
		time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC),
	)
	pinnedAt := time.Date(2026, 8, 18, 12, 1, 0, 0, time.UTC)
	candidate.PinnedAt = &pinnedAt
	snapshot := catalog.Snapshot{
		Source:     candidate.Source,
		Candidates: []catalog.Candidate{candidate},
		ObservedAt: candidate.ObservedAt,
	}

	if err := publisher.Publish(context.Background(), snapshot); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if strings.Contains(receivedJSON, "pinnedAt") || strings.Contains(receivedJSON, "\"pinned\"") {
		t.Fatalf("payload = %s, must not contain pin state", receivedJSON)
	}
}

func TestNewPublisherRejectsUnsafeGatewayURL(t *testing.T) {
	for _, gatewayURL := range []string{
		"",
		"not a URL",
		"ftp://gateway.example.test",
		"https://user:pass@gateway.example.test",
		"https://gateway.example.test/?token=secret",
	} {
		if _, err := NewPublisher(gatewayURL, "token"); err == nil {
			t.Fatalf("NewPublisher(%q) error = nil, want validation error", gatewayURL)
		}
	}
}

func TestPublisherSanitizesGatewayFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "secret gateway response", http.StatusUnauthorized)
	}))
	defer server.Close()

	publisher, err := NewPublisher(server.URL, "token")
	if err != nil {
		t.Fatalf("NewPublisher() error = %v", err)
	}
	candidate := catalog.NewCandidate(
		catalog.SourceRef{Kind: "container", ID: "docker-local"},
		catalog.ResourceRef{Kind: "container", Name: "whoami"},
		time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC),
	)
	err = publisher.Publish(context.Background(), catalog.Snapshot{
		Source:     candidate.Source,
		Candidates: []catalog.Candidate{candidate},
		ObservedAt: candidate.ObservedAt,
	})
	if err == nil || strings.Contains(err.Error(), "secret gateway response") || !strings.Contains(err.Error(), "HTTP status 401") {
		t.Fatalf("Publish() error = %v, want sanitized status error", err)
	}
}
