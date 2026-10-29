package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/araihu/balemoh/internal/api/generated"
	push "github.com/araihu/balemoh/internal/application/webpush"
)

func (h Handler) WithWebPush(service *push.Service) Handler { h.webPush = service; return h }
func (h Handler) GetWebPushSettings(w http.ResponseWriter, r *http.Request, _ generated.GetWebPushSettingsParams) {
	w.Header().Set("Cache-Control", "no-store")
	if !h.notificationOperatorAuth(w, r) {
		return
	}
	if h.webPush == nil {
		writeJSON(w, 200, push.Settings{})
		return
	}
	settings, err := h.webPush.Settings(r.Context(), r.Header.Get("X-Push-Token"))
	if err != nil {
		writePushError(w, err)
		return
	}
	writeJSON(w, 200, settings)
}
func (h Handler) RegisterWebPush(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !h.notificationOperatorAuth(w, r) {
		return
	}
	if !h.webPush.Enabled() {
		writeNotificationUnavailable(w)
		return
	}
	var registration push.Registration
	if !decodePushRequest(w, r, &registration) {
		return
	}
	if err := h.webPush.Register(r.Context(), registration); err != nil {
		writePushError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h Handler) DeleteWebPush(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !h.notificationOperatorAuth(w, r) {
		return
	}
	if h.webPush == nil {
		writeNotificationUnavailable(w)
		return
	}
	var token struct {
		Token string `json:"token"`
	}
	if !decodePushRequest(w, r, &token) {
		return
	}
	if err := h.webPush.Delete(r.Context(), token.Token); err != nil {
		writePushError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func decodePushRequest(w http.ResponseWriter, r *http.Request, value any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		writePushError(w, push.ErrInvalid)
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writePushError(w, push.ErrInvalid)
		return false
	}
	return true
}
func writePushError(w http.ResponseWriter, err error) {
	status := http.StatusServiceUnavailable
	switch {
	case errors.Is(err, push.ErrInvalid):
		status = http.StatusBadRequest
	case errors.Is(err, push.ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, push.ErrCapacity):
		status = http.StatusTooManyRequests
	}
	writeJSON(w, status, generated.ErrorResponse{Code: "web_push_unavailable", Message: "Web Push request could not be completed"})
}
