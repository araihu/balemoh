package bff

import (
	"context"
	"errors"
	"net/http"
)

func (s *server) lifecycle(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	if err := r.ParseForm(); err != nil {
		s.renderStaging(w, r, "", "Unable to read action. Try again.", 400)
		return
	}
	action := r.PostForm.Get("action")
	if action != "hide" && action != "show" && action != "purge" {
		s.renderStaging(w, r, "", "Unknown service action.", 400)
		return
	}
	if err := s.catalog.Lifecycle(ctx, r.PathValue("serviceID"), action); err != nil {
		status := http.StatusServiceUnavailable
		message := "Action could not be completed. Refresh to check the current state before retrying."
		var upstream *upstreamError
		if errors.As(err, &upstream) {
			if upstream.status == 409 {
				status = 409
				message = "Service state changed. Permanent deletion requires a missing resource. Refresh and try again."
			}
			if upstream.status == 404 {
				status = 404
				message = "This service no longer exists."
			}
		}
		s.renderStaging(w, r, "", message, status)
		return
	}
	state := "live"
	if action == "hide" {
		state = "hidden"
	}
	if action == "purge" {
		state = "missing"
	}
	if action == "show" {
		service, status := s.findService(ctx, r.PathValue("serviceID"))
		if status != http.StatusOK {
			redirect(w, r, "/staging?notice="+action)
			return
		}
		state = service.State()
	}
	redirect(w, r, "/staging?notice="+action+"&status="+state+"#"+state)
}
