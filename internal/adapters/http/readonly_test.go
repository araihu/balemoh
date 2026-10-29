package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReadOnlyOperatorBoundary(t *testing.T) {
	for _, test := range []struct {
		method, path string
		allowed      bool
	}{
		{"GET", "/api/v1/staging/services", true},
		{"HEAD", "/healthz", true},
		{"POST", "/api/v1/staging/services", false},
		{"POST", "/api/v1/discovery/sync", false},
		{"POST", "/api/v1/staging/services/a/pin", false},
		{"DELETE", "/api/v1/staging/services/a/pin", false},
		{"PUT", "/api/v1/plugin-widgets/a", false},
		{"POST", "/api/v1/notifications/read", false},
		{"DELETE", "/api/v1/notifications", false},
		{"POST", "/api/v1/federation/snapshots", true},
		{"POST", "/api/v1/federation/telemetry", true},
		{"POST", "/api/v1/plugins/p/connections/c/queries/alerts.list", true},
		{"POST", "/api/v1/plugins/p/connections/c/queries/q/extra", false},
	} {
		t.Run(test.method+test.path, func(t *testing.T) {
			called := false
			handler := ReadOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(http.StatusUnauthorized) }))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
			if called != test.allowed {
				t.Fatalf("downstream called=%v, want %v", called, test.allowed)
			}
			want := http.StatusForbidden
			if test.allowed {
				want = http.StatusUnauthorized
			}
			if response.Code != want {
				t.Fatalf("status=%d, want %d", response.Code, want)
			}
		})
	}
}
