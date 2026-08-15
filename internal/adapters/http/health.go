package http

import (
	"encoding/json"
	"net/http"

	"github.com/araihu/balemoh/internal/api/generated"
	"github.com/araihu/balemoh/internal/application/health"
)

type Handler struct {
	checker health.Checker
}

func NewHandler(checker health.Checker) generated.ServerInterface {
	return Handler{checker: checker}
}

func (h Handler) GetHealthz(w http.ResponseWriter, r *http.Request) {
	if err := h.checker.Check(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, generated.ErrorResponse{
			Code:    "unavailable",
			Message: "service unavailable",
		})
		return
	}

	writeJSON(w, http.StatusOK, generated.HealthResponse{Status: generated.Ok})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

var _ generated.ServerInterface = Handler{}
