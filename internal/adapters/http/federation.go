package http

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/araihu/balemoh/internal/api/generated"
	"github.com/araihu/balemoh/internal/application/catalog"
)

const maxFederationSnapshotBytes = 2 << 20

func (h Handler) ImportFederationSnapshot(w http.ResponseWriter, r *http.Request) {
	if h.federation == nil || h.federationToken == "" {
		writeJSON(w, http.StatusNotFound, generated.ErrorResponse{
			Code:    "federation_disabled",
			Message: "federation ingestion is not enabled",
		})
		return
	}
	if !validBearerToken(r.Header.Get("Authorization"), h.federationToken) {
		writeJSON(w, http.StatusUnauthorized, generated.ErrorResponse{
			Code:    "unauthorized",
			Message: "invalid federation credentials",
		})
		return
	}

	var payload generated.FederationSnapshot
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxFederationSnapshotBytes))
	if err := decoder.Decode(&payload); err != nil {
		writeInvalidFederationSnapshot(w)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeInvalidFederationSnapshot(w)
		return
	}
	snapshot, err := federationSnapshot(payload)
	if err != nil {
		writeInvalidFederationSnapshot(w)
		return
	}
	if _, ok := h.allowedFederated[sourceKey(snapshot.Source)]; !ok {
		writeJSON(w, http.StatusForbidden, generated.ErrorResponse{
			Code:    "source_not_registered",
			Message: "federation source is not registered",
		})
		return
	}

	result, err := h.federation.ImportSnapshot(r.Context(), snapshot)
	if err != nil {
		if errors.Is(err, catalog.ErrInvalidSnapshot) {
			writeInvalidFederationSnapshot(w)
			return
		}
		writeJSON(w, http.StatusServiceUnavailable, generated.ErrorResponse{
			Code:    "unavailable",
			Message: "catalog unavailable",
		})
		return
	}
	writeJSON(w, http.StatusOK, generated.DiscoverySyncResponse{
		Sources:    int32(result.Sources),
		Candidates: int32(result.Candidates),
	})
}

func federationSnapshot(payload generated.FederationSnapshot) (catalog.Snapshot, error) {
	source := catalog.SourceRef{Kind: payload.Source.Kind, ID: payload.Source.Id}
	candidates := make([]catalog.Candidate, 0, len(payload.Candidates))
	for _, candidate := range payload.Candidates {
		resource := catalog.ResourceRef{Kind: candidate.Resource.Kind, Name: candidate.Resource.Name}
		if candidate.Resource.Namespace != nil {
			resource.Namespace = *candidate.Resource.Namespace
		}
		endpoints := make([]catalog.Endpoint, 0, len(candidate.Endpoints))
		for _, endpoint := range candidate.Endpoints {
			endpoints = append(endpoints, catalog.Endpoint{
				Name:       endpoint.Name,
				URL:        endpoint.Url,
				Port:       int(endpoint.Port),
				Protocol:   endpoint.Protocol,
				Provenance: endpoint.Provenance,
			})
		}
		candidates = append(candidates, catalog.Candidate{
			ID:          candidate.Id,
			Source:      catalog.SourceRef{Kind: candidate.Source.Kind, ID: candidate.Source.Id},
			Resource:    resource,
			DisplayName: candidate.DisplayName,
			Description: candidate.Description,
			Metadata:    candidate.Metadata,
			Endpoints:   endpoints,
			Images:      candidate.Images,
			ObservedAt:  candidate.ObservedAt,
		})
	}
	snapshot := catalog.Snapshot{
		Source:     source,
		Candidates: candidates,
		ObservedAt: payload.ObservedAt,
	}
	if err := snapshot.Validate(); err != nil {
		return catalog.Snapshot{}, err
	}
	return snapshot, nil
}

func sourceKey(source catalog.SourceRef) string {
	return strings.TrimSpace(source.Kind) + "/" + strings.TrimSpace(source.ID)
}

func validBearerToken(header, expected string) bool {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return false
	}
	provided := []byte(parts[1])
	want := []byte(expected)
	return len(provided) == len(want) && subtle.ConstantTimeCompare(provided, want) == 1
}

func writeInvalidFederationSnapshot(w http.ResponseWriter) {
	writeJSON(w, http.StatusBadRequest, generated.ErrorResponse{
		Code:    "invalid_snapshot",
		Message: "invalid federation snapshot",
	})
}

var _ generated.ServerInterface = Handler{}
