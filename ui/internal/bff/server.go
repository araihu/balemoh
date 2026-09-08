package bff

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	api "github.com/araihu/balemoh/client"
	uiassets "github.com/araihu/balemoh/ui"
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
	mux.HandleFunc("GET /ui/icons/sprite.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = w.Write(uiassets.IconSprite)
	})
	mux.HandleFunc("GET /ui/balemoh.css", server.serveCSS)
	mux.HandleFunc("GET /healthz", server.healthz)
	mux.HandleFunc("GET /staging", server.staging)
	mux.HandleFunc("POST /staging/sync", server.sync)
	mux.HandleFunc("POST /staging/services/{serviceID}/selection", server.setPin)
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
	s.renderStaging(w, r, noticeText(r.URL.Query().Get("notice")), "", http.StatusOK)
}

func (s *server) sync(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	if _, err := s.catalog.Sync(ctx); err != nil {
		s.renderStaging(w, r, "", pageSyncError, http.StatusServiceUnavailable)
		return
	}
	redirect(w, r, "/staging?notice=synced")
}

func (s *server) setPin(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	if err := r.ParseForm(); err != nil {
		s.renderStaging(w, r, "", "Unable to read the pin change. Try again.", http.StatusBadRequest)
		return
	}
	values := r.PostForm["pinned"]
	if len(values) > 1 || (len(values) == 1 && values[0] != "true") {
		s.renderStaging(w, r, "", "Invalid pin state. Try again.", http.StatusBadRequest)
		return
	}
	desired := len(values) == 1
	id := r.PathValue("serviceID")
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	services, err := s.catalog.Staging(ctx)
	if err != nil {
		s.renderStaging(w, r, "", pageErrorMessage, http.StatusServiceUnavailable)
		return
	}
	for _, service := range services {
		if service.Id != id {
			continue
		}
		if service.Pinned != desired {
			if desired {
				err = s.catalog.Pin(ctx, id)
			} else {
				err = s.catalog.Unpin(ctx, id)
			}
			if err != nil {
				s.renderStaging(w, r, "", pageMutationError, http.StatusServiceUnavailable)
				return
			}
		}
		notice := "unpinned"
		if desired {
			notice = "pinned"
		}
		redirect(w, r, "/staging?notice="+notice+"#select-"+url.PathEscape(id))
		return
	}
	s.renderStaging(w, r, "", "This service is no longer available. Refresh discovery.", http.StatusConflict)
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
		s.renderStaging(w, r, "", pageMutationError, http.StatusServiceUnavailable)
		return
	}
	redirect(w, r, "/staging?notice="+notice)
}

func (s *server) renderStaging(w http.ResponseWriter, r *http.Request, notice, errorMessage string, status int) {
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	services, err := s.catalog.Staging(ctx)
	if err != nil && errorMessage == "" {
		errorMessage = pageErrorMessage
	}
	if err != nil {
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
	w.Header().Set("Cache-Control", "no-store")
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
	resources := make([]view.Service, 0)
	if service.Resources != nil {
		for _, member := range *service.Resources {
			resources = append(resources, mapService(api.ServiceCandidate{Resource: member.Resource, Source: service.Source, DisplayName: member.Resource.Name, Endpoints: member.Endpoints, Images: member.Images}))
		}
	}
	return view.Service{
		Resources:   resources,
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
