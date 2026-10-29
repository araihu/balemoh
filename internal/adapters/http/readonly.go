package http

import (
	"net/http"
	"strings"
)

// ReadOnly denies operator writes while preserving authenticated federation
// imports and retained plugin queries. It must wrap the final plugin router.
func ReadOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowed := r.Method == http.MethodGet || r.Method == http.MethodHead
		if r.Method == http.MethodPost {
			parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
			allowed = r.URL.Path == "/api/v1/federation/snapshots" || r.URL.Path == "/api/v1/federation/telemetry" ||
				(len(parts) == 8 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "plugins" && parts[4] == "connections" && parts[6] == "queries")
		}
		if !allowed {
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, http.StatusForbidden, map[string]string{"code": "read_only", "message": "This instance is read-only."})
			return
		}
		next.ServeHTTP(w, r)
	})
}
