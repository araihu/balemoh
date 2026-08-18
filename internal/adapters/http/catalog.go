package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/araihu/balemoh/internal/api/generated"
	"github.com/araihu/balemoh/internal/application/catalog"
)

func (h Handler) GetStagingServices(w http.ResponseWriter, r *http.Request) {
	services, err := h.catalog.ListStaging(r.Context())
	if err != nil {
		writeCatalogUnavailable(w)
		return
	}
	writeJSON(w, http.StatusOK, generated.ServiceListResponse{Services: serviceCandidates(services)})
}

func (h Handler) GetHomepageServices(w http.ResponseWriter, r *http.Request) {
	services, err := h.catalog.ListHomepage(r.Context())
	if err != nil {
		writeCatalogUnavailable(w)
		return
	}
	writeJSON(w, http.StatusOK, generated.ServiceListResponse{Services: serviceCandidates(services)})
}

func (h Handler) PinStagingService(w http.ResponseWriter, r *http.Request, serviceID string) {
	service, err := h.catalog.Pin(r.Context(), serviceID)
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, serviceCandidate(service))
}

func (h Handler) UnpinStagingService(w http.ResponseWriter, r *http.Request, serviceID string) {
	service, err := h.catalog.Unpin(r.Context(), serviceID)
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, serviceCandidate(service))
}

func (h Handler) SyncDiscovery(w http.ResponseWriter, r *http.Request) {
	result, err := h.catalog.Sync(r.Context())
	if err != nil {
		writeCatalogUnavailable(w)
		return
	}
	writeJSON(w, http.StatusOK, generated.DiscoverySyncResponse{
		Sources:    int32(result.Sources),
		Candidates: int32(result.Candidates),
	})
}

func serviceCandidates(candidates []catalog.Candidate) []generated.ServiceCandidate {
	services := make([]generated.ServiceCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		services = append(services, serviceCandidate(candidate))
	}
	return services
}

func serviceCandidate(candidate catalog.Candidate) generated.ServiceCandidate {
	metadata := make(map[string]string, len(candidate.Metadata))
	for key, value := range candidate.Metadata {
		metadata[key] = value
	}
	endpoints := make([]generated.ServiceEndpoint, 0, len(candidate.Endpoints))
	for _, endpoint := range candidate.Endpoints {
		endpoints = append(endpoints, generated.ServiceEndpoint{
			Name:       endpoint.Name,
			Url:        endpoint.URL,
			Port:       int32(endpoint.Port),
			Protocol:   endpoint.Protocol,
			Provenance: endpoint.Provenance,
		})
	}
	images := append([]string(nil), candidate.Images...)
	resource := generated.ResourceRef{Kind: candidate.Resource.Kind, Name: candidate.Resource.Name}
	if candidate.Resource.Namespace != "" {
		namespace := candidate.Resource.Namespace
		resource.Namespace = &namespace
	}
	var pinnedAt *time.Time
	if candidate.PinnedAt != nil {
		value := candidate.PinnedAt.UTC()
		pinnedAt = &value
	}
	return generated.ServiceCandidate{
		Id:          candidate.ID,
		Source:      generated.SourceRef{Kind: candidate.Source.Kind, Id: candidate.Source.ID},
		Resource:    resource,
		DisplayName: candidate.DisplayName,
		Description: candidate.Description,
		Metadata:    metadata,
		Endpoints:   endpoints,
		Images:      images,
		ObservedAt:  candidate.ObservedAt.UTC(),
		Pinned:      candidate.PinnedAt != nil,
		PinnedAt:    pinnedAt,
	}
}

func writeCatalogError(w http.ResponseWriter, err error) {
	if errors.Is(err, catalog.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, generated.ErrorResponse{
			Code:    "not_found",
			Message: "service candidate not found",
		})
		return
	}
	writeCatalogUnavailable(w)
}

func writeCatalogUnavailable(w http.ResponseWriter) {
	writeJSON(w, http.StatusServiceUnavailable, generated.ErrorResponse{
		Code:    "unavailable",
		Message: "catalog unavailable",
	})
}

var _ generated.ServerInterface = Handler{}
