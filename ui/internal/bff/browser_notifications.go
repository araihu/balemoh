package bff

import (
	"context"
	"encoding/json"
	api "github.com/araihu/balemoh/client"
	"io"
	"net/http"
)

type WebPushCatalog interface {
	WebPushSettings(context.Context, string) (api.WebPushSettings, error)
	RegisterWebPush(context.Context, api.WebPushRegistration) error
	DeleteWebPush(context.Context, api.WebPushToken) error
}

func (c *APIClient) WebPushSettings(ctx context.Context, token string) (api.WebPushSettings, error) {
	auth, err := c.notificationAuthority()
	if err != nil {
		return api.WebPushSettings{}, err
	}
	params := &api.GetWebPushSettingsParams{}
	if token != "" {
		params.XPushToken = &token
	}
	response, err := c.client.GetWebPushSettingsWithResponse(ctx, params, auth)
	if err != nil {
		return api.WebPushSettings{}, &upstreamError{operation: "Web Push", cause: err}
	}
	if response.StatusCode() != 200 || response.JSON200 == nil {
		return api.WebPushSettings{}, &upstreamError{operation: "Web Push", status: response.StatusCode()}
	}
	return *response.JSON200, nil
}
func (c *APIClient) RegisterWebPush(ctx context.Context, registration api.WebPushRegistration) error {
	auth, err := c.notificationAuthority()
	if err != nil {
		return err
	}
	response, err := c.client.RegisterWebPushWithResponse(ctx, registration, auth)
	if err != nil {
		return &upstreamError{operation: "Web Push", cause: err}
	}
	if response.StatusCode() != 204 {
		return &upstreamError{operation: "Web Push", status: response.StatusCode()}
	}
	return nil
}
func (c *APIClient) DeleteWebPush(ctx context.Context, token api.WebPushToken) error {
	auth, err := c.notificationAuthority()
	if err != nil {
		return err
	}
	response, err := c.client.DeleteWebPushWithResponse(ctx, token, auth)
	if err != nil {
		return &upstreamError{operation: "Web Push", cause: err}
	}
	if response.StatusCode() != 204 {
		return &upstreamError{operation: "Web Push", status: response.StatusCode()}
	}
	return nil
}
func (s *server) webPush(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	client, ok := s.catalog.(WebPushCatalog)
	if !ok {
		http.Error(w, "Web Push unavailable.", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	var err error
	switch r.Method {
	case "GET":
		token := r.Header.Get("X-Push-Token")
		if len(token) > 43 {
			http.Error(w, "Invalid browser token.", 400)
			return
		}
		var settings api.WebPushSettings
		settings, err = client.WebPushSettings(ctx, token)
		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(settings)
			return
		}
	case "POST":
		var registration api.WebPushRegistration
		if !decodeWebPush(w, r, &registration) {
			return
		}
		err = client.RegisterWebPush(ctx, registration)
	case "DELETE":
		var token api.WebPushToken
		if !decodeWebPush(w, r, &token) {
			return
		}
		err = client.DeleteWebPush(ctx, token)
	}
	if err != nil {
		http.Error(w, "Web Push request could not be confirmed. Retry.", 503)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func decodeWebPush(w http.ResponseWriter, r *http.Request, value any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		http.Error(w, "Invalid Web Push request.", 400)
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		http.Error(w, "Invalid Web Push request.", 400)
		return false
	}
	return true
}
