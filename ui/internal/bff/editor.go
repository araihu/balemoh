package bff

import (
	"context"
	"net/http"
	"net/url"
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
	if values, ok := r.PostForm["icon"]; ok && len(values) == 1 {
		service.IconRef = values[0]
		service.Icon = resolveIcon(values[0])
		if values[0] == "" {
			service.Icon = service.DefaultIcon
		}
	}
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
	if err := s.catalog.Edit(ctx, service.ID, api.ServiceEdit{DisplayName: service.DisplayName, Description: service.Description, Address: service.Address, Icon: &service.IconRef}); err != nil {
		service.EditError = "Changes could not be saved. Your draft is still here; try again."
		s.renderEditor(w, r, service, http.StatusServiceUnavailable)
		return
	}
	redirect(w, r, "/staging?notice=edited")
}

func (s *server) renderEditor(w http.ResponseWriter, r *http.Request, service view.Service, status int) {
	title := service.DisplayName
	if title == "" {
		title = "Service details"
	}
	s.render(w, r, view.PageData{Title: title, Description: "Review discovered resources and edit this service's name, description, address, and icon.", Path: view.EditURL(service.ID), Active: "nav-staging", Staging: true, Editor: &service}, status)
}
