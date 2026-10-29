package bff

import (
	"net/http"

	"github.com/araihu/balemoh/ui/internal/view"
)

func (s *server) attributions(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, view.PageData{
		Title:        "Attributions",
		Description:  "Credits, sources, and license notices for Balemoh's icons and artwork.",
		Path:         "/attributions",
		Attributions: true,
	}, http.StatusOK)
}
