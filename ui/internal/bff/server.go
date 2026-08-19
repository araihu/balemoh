package bff

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	api "github.com/araihu/balemoh/client"
	"github.com/araihu/balemoh/ui/internal/view"
	shellassets "github.com/araihu/goshtoso-app-shells/consoleshell/assets"
	"github.com/araihu/goshtoso/assets"
)

const (
	pageErrorMessage  = "Não foi possível consultar o catálogo de serviços. Tente novamente."
	pageSyncError     = "Não foi possível atualizar a descoberta. Tente novamente."
	pageMutationError = "Não foi possível atualizar o serviço. Tente novamente."
)

// New returns the private, server-rendered UI BFF handler.
func New(catalog Catalog, requestTimeout time.Duration) (http.Handler, error) {
	if catalog == nil {
		return nil, errors.New("catalog client is required")
	}
	if requestTimeout <= 0 {
		return nil, errors.New("request timeout must be positive")
	}

	server := &server{catalog: catalog, timeout: requestTimeout}
	mux := http.NewServeMux()
	mux.Handle("GET /assets/", assets.Handler())
	mux.Handle("GET /consoleshell/assets/", shellassets.Handler())
	mux.HandleFunc("GET /ui/balemoh.css", server.serveCSS)
	mux.HandleFunc("GET /healthz", server.healthz)
	mux.HandleFunc("GET /staging", server.staging)
	mux.HandleFunc("POST /staging/sync", server.sync)
	mux.HandleFunc("POST /staging/services/{serviceID}/pin", server.pin)
	mux.HandleFunc("POST /staging/services/{serviceID}/unpin", server.unpin)
	mux.HandleFunc("GET /", server.homepage)
	return mux, nil
}

type server struct {
	catalog Catalog
	timeout time.Duration
}

func (s *server) homepage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	services, err := s.catalog.Homepage(ctx)
	if err != nil {
		s.render(w, r, view.PageData{
			Title:       "Homepage",
			Description: "The services pinned for quick access across your homelab.",
			Active:      "nav-home",
			Error:       pageErrorMessage,
		}, http.StatusServiceUnavailable)
		return
	}
	s.render(w, r, view.PageData{
		Title:       "Homepage",
		Description: "The services pinned for quick access across your homelab.",
		Active:      "nav-home",
		Services:    mapServices(services),
		Notice:      noticeText(r.URL.Query().Get("notice")),
	}, http.StatusOK)
}

func (s *server) staging(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/staging" {
		http.NotFound(w, r)
		return
	}
	s.renderStaging(w, r, noticeText(r.URL.Query().Get("notice")), "")
}

func (s *server) sync(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	if _, err := s.catalog.Sync(ctx); err != nil {
		s.renderStaging(w, r, "", pageSyncError)
		return
	}
	redirect(w, r, "/staging?notice=synced")
}

func (s *server) pin(w http.ResponseWriter, r *http.Request) {
	s.mutate(w, r, func(ctx context.Context, id string) error {
		return s.catalog.Pin(ctx, id)
	}, "pinned")
}

func (s *server) unpin(w http.ResponseWriter, r *http.Request) {
	s.mutate(w, r, func(ctx context.Context, id string) error {
		return s.catalog.Unpin(ctx, id)
	}, "unpinned")
}

func (s *server) mutate(w http.ResponseWriter, r *http.Request, operation func(context.Context, string) error, notice string) {
	id := strings.TrimSpace(r.PathValue("serviceID"))
	if id == "" {
		http.Error(w, "service ID is required", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	if err := operation(ctx, id); err != nil {
		s.renderStaging(w, r, "", pageMutationError)
		return
	}
	redirect(w, r, "/staging?notice="+notice)
}

func (s *server) renderStaging(w http.ResponseWriter, r *http.Request, notice, errorMessage string) {
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	services, err := s.catalog.Staging(ctx)
	if err != nil && errorMessage == "" {
		errorMessage = pageErrorMessage
	}
	status := http.StatusOK
	if err != nil || errorMessage != "" {
		status = http.StatusServiceUnavailable
	}
	s.render(w, r, view.PageData{
		Title:       "Staging",
		Description: "Review discovered candidates and pin the services that belong on your homepage.",
		Active:      "nav-staging",
		Services:    mapServices(services),
		Staging:     true,
		Error:       errorMessage,
		Notice:      notice,
	}, status)
}

func (s *server) healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *server) serveCSS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(view.BalemohCSS())
}

func (s *server) render(w http.ResponseWriter, r *http.Request, data view.PageData, status int) {
	var body bytes.Buffer
	if err := view.ConsolePage(data).Render(r.Context(), &body); err != nil {
		http.Error(w, "unable to render UI", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
	_, _ = io.Copy(w, &body)
}

func redirect(w http.ResponseWriter, _ *http.Request, location string) {
	w.Header().Set("Location", location)
	w.WriteHeader(http.StatusSeeOther)
}

func noticeText(value string) string {
	switch value {
	case "pinned":
		return "The service is now visible on the homepage."
	case "unpinned":
		return "The service was removed from the homepage."
	case "synced":
		return "Discovery completed; the candidate list is current."
	default:
		return ""
	}
}

func mapServices(services []api.ServiceCandidate) []view.Service {
	result := make([]view.Service, 0, len(services))
	for _, service := range services {
		result = append(result, mapService(service))
	}
	return result
}

func mapService(service api.ServiceCandidate) view.Service {
	namespace := ""
	if service.Resource.Namespace != nil {
		namespace = strings.TrimSpace(*service.Resource.Namespace)
	}
	displayName := strings.TrimSpace(service.DisplayName)
	if displayName == "" {
		displayName = service.Resource.Name
	}
	if displayName == "" {
		displayName = service.Id
	}
	resource := strings.Trim(strings.TrimSpace(service.Resource.Kind)+"/"+strings.TrimSpace(service.Resource.Name), "/")
	source := strings.Trim(strings.TrimSpace(service.Source.Kind)+"/"+strings.TrimSpace(service.Source.Id), "/")
	endpoints := make([]view.Endpoint, 0, len(service.Endpoints))
	for _, endpoint := range service.Endpoints {
		endpoints = append(endpoints, view.Endpoint{
			Name:       endpoint.Name,
			URL:        endpoint.Url,
			Protocol:   endpoint.Protocol,
			Port:       endpoint.Port,
			Provenance: endpoint.Provenance,
		})
	}
	return view.Service{
		ID:          service.Id,
		DisplayName: displayName,
		Description: service.Description,
		Source:      source,
		Resource:    resource,
		Namespace:   namespace,
		Pinned:      service.Pinned,
		Endpoints:   endpoints,
		Images:      append([]string(nil), service.Images...),
	}
}
