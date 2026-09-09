package bff

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	api "github.com/araihu/balemoh/client"
	"github.com/araihu/balemoh/ui/internal/view"
)

func (s *server) findService(ctx context.Context, id string) (view.Service, int) {
	services, err := s.catalog.Staging(ctx)
	if err != nil {
		return view.Service{ID: id, EditError: pageErrorMessage}, http.StatusServiceUnavailable
	}
	for _, service := range services {
		if service.Id == id {
			return mapService(service), http.StatusOK
		}
	}
	return view.Service{ID: id, EditError: "This service is no longer available."}, http.StatusNotFound
}

func (s *server) edit(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	service, status := s.findService(ctx, r.PathValue("serviceID"))
	s.renderEditor(w, r, service, status)
}

func (s *server) saveEdit(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	service, status := s.findService(ctx, r.PathValue("serviceID"))
	if status != http.StatusOK {
		s.renderEditor(w, r, service, status)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	if err := r.ParseForm(); err != nil {
		service.EditError = "Unable to read changes. Try again."
		s.renderEditor(w, r, service, http.StatusBadRequest)
		return
	}
	service.DisplayName = strings.TrimSpace(r.PostForm.Get("displayName"))
	service.Description = strings.TrimSpace(r.PostForm.Get("description"))
	service.Address = strings.TrimSpace(r.PostForm.Get("address"))
	if service.DisplayName == "" || utf8.RuneCountInString(service.DisplayName) > 200 || utf8.RuneCountInString(service.Description) > 2000 || len(service.Address) > 2048 {
		service.EditError = "Enter a name up to 200 characters, a description up to 2000 characters, and an address up to 2048 bytes."
	}
	if service.Address != "" {
		u, err := url.Parse(service.Address)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil {
			service.EditError = "Use an absolute HTTP or HTTPS address without credentials, or leave it blank."
		}
	}
	if service.EditError != "" {
		s.renderEditor(w, r, service, http.StatusBadRequest)
		return
	}
	if err := s.catalog.Edit(ctx, service.ID, api.ServiceEdit{DisplayName: service.DisplayName, Description: service.Description, Address: service.Address}); err != nil {
		service.EditError = "Changes could not be saved. Your draft is still here; try again."
		s.renderEditor(w, r, service, http.StatusServiceUnavailable)
		return
	}
	if r.Header.Get("HX-Request") != "true" {
		redirect(w, r, "/staging?notice=edited")
		return
	}
	var body bytes.Buffer
	if err := view.EditedRow(service).Render(r.Context(), &body); err != nil {
		http.Error(w, "Unable to render saved service. Refresh staging.", 500)
		return
	}
	events, _ := json.Marshal(map[string]any{"drawer:close": map[string]string{"id": "serviceEditor"}, "service-edited": map[string]string{"id": service.ID}, "notify": map[string]string{"kind": "toast", "tone": "success", "message": "Service updated."}})
	w.Header().Set("HX-Trigger-After-Settle", string(events))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(body.Bytes())
}

func (s *server) renderEditor(w http.ResponseWriter, r *http.Request, service view.Service, status int) {
	if r.Header.Get("HX-Request") != "true" {
		service.Standalone = true
		s.render(w, r, view.PageData{Title: "Edit service", Description: "Edit the service name, description, and homepage address.", Path: view.EditURL(service.ID), Active: "nav-staging", Staging: true, Editor: &service}, status)
		return
	}
	var body bytes.Buffer
	if err := view.ServiceEditor(service).Render(r.Context(), &body); err != nil {
		http.Error(w, "Unable to render editor.", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Balemoh-Status", strconv.Itoa(status))
	w.Header().Set("HX-Trigger-After-Swap", `{"drawer:open":{"id":"serviceEditor"}}`)
	_, _ = w.Write(body.Bytes())
}
