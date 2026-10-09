package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/araihu/balemoh/internal/adapters/sqlite"
	"github.com/araihu/balemoh/internal/api/generated"
	"github.com/araihu/balemoh/internal/application/catalog"
	"github.com/araihu/balemoh/internal/application/telemetry"
)

func TestTelemetrySourceAuthenticationAndContract(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, t.TempDir()+"/telemetry.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = sqlite.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	store := sqlite.NewTelemetryStore(db)
	source := telemetry.Source{Kind: "container", ID: "raspi"}
	handler := NewHandlerWithFederation(nil, nil, nil, map[catalog.SourceRef]string{{Kind: "container", ID: "raspi"}: "raspi-token", {Kind: "container", ID: "bastion"}: "bastion-token"}).WithTelemetry(store)
	mux := generated.HandlerFromMux(handler, http.NewServeMux())
	batch := telemetry.Batch{Source: source, ObservedAt: time.Now().UTC(), Hosts: []telemetry.Observation{{}}}
	raw, _ := json.Marshal(batch)
	for _, tc := range []struct {
		token  string
		status int
	}{{"", 401}, {"Bearer bastion-token", 401}, {"Bearer raspi-token", 204}} {
		r := httptest.NewRequest("POST", "/api/v1/federation/telemetry", strings.NewReader(string(raw)))
		r.Header.Set("Authorization", tc.token)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("auth status %d want %d: %s", w.Code, tc.status, w.Body.String())
		}
	}
	for _, body := range []string{string(raw) + " {}", strings.Replace(string(raw), `"cpuPercent":null`, `"cpuPercent":101`, 1)} {
		r := httptest.NewRequest("POST", "/api/v1/federation/telemetry", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer raspi-token")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatalf("invalid payload = %d", w.Code)
		}
	}
	oversized := httptest.NewRequest("POST", "/api/v1/federation/telemetry", strings.NewReader(string(raw)+strings.Repeat(" ", 2<<20)))
	oversized.Header.Set("Authorization", "Bearer raspi-token")
	largeResponse := httptest.NewRecorder()
	mux.ServeHTTP(largeResponse, oversized)
	if largeResponse.Code != 413 {
		t.Fatalf("body limit = %d", largeResponse.Code)
	}
	id := telemetry.ID(source, "")
	for path, status := range map[string]int{"/api/v1/hosts": 200, "/api/v1/hosts/" + id + "?range=24h": 200, "/api/v1/hosts/" + id + "?range=7d": 400, "/api/v1/hosts/unknown": 404} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != status {
			t.Errorf("%s = %d", path, w.Code)
		}
		if strings.HasSuffix(path, "24h") {
			var detail generated.HostDetail
			if err = json.Unmarshal(w.Body.Bytes(), &detail); err != nil || len(detail.Samples) != 1440 || detail.Host.Status != "partial" {
				t.Fatalf("generated contract: %v", err)
			}
		}
	}
}
