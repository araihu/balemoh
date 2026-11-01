package bff

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	api "github.com/araihu/balemoh/client"
	"github.com/araihu/balemoh/ui/internal/view"
	"net/http"
	"time"
)

func (s *server) layoutEditorEvents(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.catalog.(LayoutCatalog); !ok {
		http.Error(w, "Layout updates unavailable.", 503)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	control := http.NewResponseController(w)
	defer control.SetWriteDeadline(time.Time{})
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	var previous [32]byte
	for {
		ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
		snapshot, e := s.dashboardSnapshot(ctx)
		cancel()
		var payload []byte
		if e == nil && snapshot.LayoutRevision != nil {
			identities := []api.DashboardWidget{}
			for _, item := range snapshot.Items {
				item.Data = nil
				item.Error = ""
				identities = append(identities, item)
			}
			raw, _ := json.Marshal(identities)
			identity := sha256.Sum256(raw)
			var html bytes.Buffer
			if e = view.LayoutLiveWidgets(renderWidgets(snapshot, s.widgets).Widgets).Render(r.Context(), &html); e != nil {
				return
			}
			payload, _ = json.Marshal(map[string]any{"revision": *snapshot.LayoutRevision, "identity": hex.EncodeToString(identity[:]), "html": html.String()})
		}
		digest := sha256.Sum256(payload)
		frame := []byte(": keepalive\n\n")
		if len(payload) > 0 && digest != previous {
			frame = append(append([]byte("data: "), payload...), []byte("\n\n")...)
			previous = digest
		}
		_ = control.SetWriteDeadline(time.Now().Add(s.timeout))
		if _, e = w.Write(frame); e != nil {
			return
		}
		if e = control.Flush(); e != nil {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}
