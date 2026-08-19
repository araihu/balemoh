package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/araihu/balemoh/internal/api/generated"
	"github.com/araihu/balemoh/internal/application/catalog"
	"github.com/araihu/balemoh/internal/application/health"
)

type Handler struct {
	checker          health.Checker
	catalog          catalog.UseCase
	federation       catalog.SnapshotImporter
	federationTokens map[catalog.SourceRef]string
}

func NewHandler(checker health.Checker, catalogService catalog.UseCase) generated.ServerInterface {
	return Handler{checker: checker, catalog: catalogService}
}

func NewHandlerWithFederation(checker health.Checker, catalogService catalog.UseCase, importer catalog.SnapshotImporter, sourceTokens map[catalog.SourceRef]string) generated.ServerInterface {
	tokens := make(map[catalog.SourceRef]string, len(sourceTokens))
	for source, token := range sourceTokens {
		source.Kind = strings.TrimSpace(source.Kind)
		source.ID = strings.TrimSpace(source.ID)
		token = strings.TrimSpace(token)
		if source.Kind != "" && source.ID != "" && token != "" {
			tokens[source] = token
		}
	}
	return Handler{
		checker:          checker,
		catalog:          catalogService,
		federation:       importer,
		federationTokens: tokens,
	}
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
