package federation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/araihu/balemoh/internal/application/telemetry"
)

func TestTelemetryPublisherUsesSeparateEndpointAndDoesNotFollowRedirect(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/prefix/api/v1/federation/telemetry" || r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("telemetry route or credential changed")
		}
		var b telemetry.Batch
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.Source.ID != "agent" {
			t.Error("invalid telemetry payload")
		}
		if calls == 1 {
			w.WriteHeader(204)
		} else {
			w.Header().Set("Location", "/must-not-follow")
			w.WriteHeader(307)
		}
	}))
	defer server.Close()
	p, err := NewPublisherWithOptions(server.URL+"/prefix", "test-token", true)
	if err != nil {
		t.Fatal(err)
	}
	b := telemetry.Batch{Source: telemetry.Source{Kind: "container", ID: "agent"}, ObservedAt: time.Now().UTC(), Hosts: []telemetry.Observation{{}}}
	if err = p.PublishTelemetry(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	if p.PublishTelemetry(context.Background(), b) == nil || calls != 2 {
		t.Fatal("telemetry followed redirect or accepted it")
	}
}
