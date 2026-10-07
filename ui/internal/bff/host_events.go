package bff

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/a-h/templ"
	api "github.com/araihu/balemoh/client"
	"github.com/araihu/balemoh/ui/internal/view"
)

func (s *server) hostEvents(w http.ResponseWriter, r *http.Request) {
	s.streamHost(w, r, 5*time.Second)
}

// Each connection receives full snapshots, so reconnecting needs no event log.
// ponytail: one API poll per open page; share polling if concurrent viewers grow.
func (s *server) streamHost(w http.ResponseWriter, r *http.Request, interval time.Duration) {
	window := r.URL.Query().Get("range")
	if window == "" {
		window = "1h"
	}
	if window != "1h" && window != "24h" {
		http.Error(w, "Choose a range of 1h or 24h.", http.StatusBadRequest)
		return
	}
	catalog, ok := s.catalog.(HostCatalog)
	if !ok {
		http.Error(w, "Live updates unavailable. Refresh to reconnect.", http.StatusServiceUnavailable)
		return
	}
	fetch := func() (api.HostDetail, error) {
		ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
		defer cancel()
		return catalog.Host(ctx, r.PathValue("hostID"), window)
	}
	detail, err := fetch()
	if err != nil {
		var upstream *upstreamError
		if errors.As(err, &upstream) && upstream.status == 404 {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Live updates unavailable. Refresh to reconnect.", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	controller := http.NewResponseController(w)
	defer controller.SetWriteDeadline(time.Time{})
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var previous [sha256.Size]byte
	failed := false
	for {
		var component templ.Component
		if err != nil {
			// Keep the last measurements visible while the upstream recovers.
			if !failed {
				component = view.HostLiveStatus("Updates unavailable. Retrying; showing last received values.")
			}
			failed = true
		} else {
			encoded, encodeErr := json.Marshal(detail)
			if encodeErr != nil {
				return
			}
			digest := sha256.Sum256(encoded)
			if digest != previous || failed {
				component = view.HostLiveSnapshot(hostDetailView(detail, window))
				previous = digest
			}
			failed = false
		}
		var frame bytes.Buffer
		if component == nil {
			frame.WriteString(": keepalive\n\n")
		} else {
			var content bytes.Buffer
			if err := component.Render(r.Context(), &content); err != nil {
				return
			}
			for line := range bytes.SplitSeq(content.Bytes(), []byte("\n")) {
				frame.WriteString("data: ")
				frame.Write(line)
				frame.WriteByte('\n')
			}
			frame.WriteByte('\n')
		}
		if err := controller.SetWriteDeadline(time.Now().Add(s.timeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
			return
		}
		if _, err := w.Write(frame.Bytes()); err != nil {
			return
		}
		if err := controller.Flush(); err != nil {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			detail, err = fetch()
		}
	}
}
