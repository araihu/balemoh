package bff

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/araihu/balemoh/ui/internal/view"
)

// PublicReadOnly applies a fixed global budget, independent of visitor identity.
// It allocates no visitor sessions and never trusts client IP headers.
func PublicReadOnly(next http.Handler, publicURL string) http.Handler {
	slots := make(chan struct{}, 16)
	var mu sync.Mutex
	tokens, last := 40.0, time.Now()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path == "/healthz" && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "This shared demo is read-only.", http.StatusForbidden)
			return
		}
		if len(r.RequestURI) > 4096 {
			http.Error(w, "Query too long.", http.StatusRequestURITooLong)
			return
		}
		p := r.URL.Path
		if p == "/dashboard/events" || strings.HasSuffix(p, "/events") {
			http.Error(w, "Live streams are disabled in this demo.", http.StatusNotFound)
			return
		}
		if p == "/staging/services/new" || strings.HasSuffix(p, "/edit") ||
			p == "/notifications/clear" || p == "/presentation" || p == "/icons" || p == "/plugins/widgets" || strings.HasPrefix(p, "/presentation/themes/editor") ||
			strings.HasSuffix(p, "/new") {
			http.Error(w, "Editing is disabled in this shared demo.", http.StatusForbidden)
			return
		}
		mu.Lock()
		now := time.Now()
		tokens = min(40, tokens+now.Sub(last).Seconds()*20)
		last = now
		allowed := tokens >= 1
		if allowed {
			tokens--
		}
		mu.Unlock()
		if !allowed {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "Demo is busy. Please retry.", http.StatusTooManyRequests)
			return
		}
		select {
		case slots <- struct{}{}:
		default:
			w.Header().Set("Retry-After", "1")
			http.Error(w, "Demo is busy. Please retry.", http.StatusTooManyRequests)
			return
		}
		// Keep the slot until the actual handler returns, including after timeout.
		// TimeoutHandler can return while its child is still cancelling upstream work.
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() { <-slots }()
			next.ServeHTTP(w, r.WithContext(view.WithReadOnly(r.Context(), publicURL)))
		})
		cacheable := r.URL.RawQuery == "" && (strings.HasPrefix(p, "/assets/") || strings.HasPrefix(p, "/consoleshell/assets/") || (strings.HasPrefix(p, "/ui/") && (strings.HasSuffix(p, ".js") || p == "/ui/balemoh.css")))
		http.TimeoutHandler(inner, 8*time.Second, "Demo request timed out. Please retry.").ServeHTTP(&demoResponse{ResponseWriter: w, cacheable: cacheable}, r)
	})
}

type demoResponse struct {
	http.ResponseWriter
	cacheable, written bool
}

func (w *demoResponse) WriteHeader(status int) {
	if w.written {
		return
	}
	w.written = true
	if w.cacheable && status == http.StatusOK && w.Header().Get("Set-Cookie") == "" {
		w.Header().Set("Cache-Control", "public, max-age=300")
	}
	w.ResponseWriter.WriteHeader(status)
}
func (w *demoResponse) Write(body []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}
