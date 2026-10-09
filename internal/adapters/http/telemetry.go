package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/araihu/balemoh/internal/api/generated"
	"github.com/araihu/balemoh/internal/application/catalog"
	"github.com/araihu/balemoh/internal/application/telemetry"
)

type TelemetryStore interface {
	List(context.Context, time.Time) ([]telemetry.Host, error)
	Detail(context.Context, string, string, time.Time) (telemetry.Detail, error)
	Ingest(context.Context, telemetry.Batch, time.Time) error
}

func (h Handler) WithTelemetry(store TelemetryStore) Handler { h.telemetry = store; return h }
func (h Handler) GetHosts(w http.ResponseWriter, r *http.Request) {
	if h.telemetry == nil {
		writeCatalogUnavailable(w)
		return
	}
	hosts, err := h.telemetry.List(r.Context(), time.Now())
	if err != nil {
		writeCatalogUnavailable(w)
		return
	}
	roots := []telemetry.Host{}
	for _, host := range hosts {
		if host.ParentID == "" {
			roots = append(roots, host)
		}
	}
	writeJSON(w, 200, struct {
		Hosts []telemetry.Host `json:"hosts"`
	}{roots})
}
func (h Handler) GetHost(w http.ResponseWriter, r *http.Request, id string, params generated.GetHostParams) {
	window := "1h"
	if params.Range != nil {
		window = string(*params.Range)
	}
	if window != "1h" && window != "24h" {
		http.Error(w, "invalid range", 400)
		return
	}
	if h.telemetry == nil {
		writeCatalogUnavailable(w)
		return
	}
	result, err := h.telemetry.Detail(r.Context(), id, window, time.Now())
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		writeCatalogUnavailable(w)
		return
	}
	writeJSON(w, 200, result)
}
func (h Handler) ImportTelemetry(w http.ResponseWriter, r *http.Request) {
	if h.telemetry == nil {
		writeCatalogUnavailable(w)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var batch telemetry.Batch
	if err := decoder.Decode(&batch); err != nil {
		status := 400
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			status = 413
		}
		http.Error(w, "invalid telemetry body", status)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		status := 400
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			status = 413
		}
		http.Error(w, "invalid telemetry body", status)
		return
	}
	token, registered := h.federationTokens[catalog.SourceRef{Kind: batch.Source.Kind, ID: batch.Source.ID}]
	if !registered {
		status := 403
		if r.Header.Get("Authorization") == "" {
			status = 401
		}
		http.Error(w, "source not authorized", status)
		return
	}
	if !validBearerToken(r.Header.Get("Authorization"), token) {
		http.Error(w, "source not authorized", 401)
		return
	}
	now := time.Now()
	if err := batch.Validate(now); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if err := h.telemetry.Ingest(r.Context(), batch, now); err != nil {
		if errors.Is(err, telemetry.ErrHostLimit) {
			http.Error(w, err.Error(), 400)
			return
		}
		writeCatalogUnavailable(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
