package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"uuid"

	"github.com/araihu/balemoh/internal/api/generated"
	"github.com/araihu/balemoh/internal/application/dashboard"
	"github.com/araihu/balemoh/internal/application/plugins"
	"github.com/araihu/balemoh/plugin-sdk/credential"
)

func MountPlugins(base http.Handler, service *plugins.Service, token string, providers ...dashboard.Provider) http.Handler {
	h := Handler{plugins: service, pluginToken: token}
	if len(providers) > 0 {
		h.dashboard = dashboard.New(append(providers, plugins.Provider{Service: service})...)
	}
	routes := generated.HandlerFromMux(h, http.NewServeMux())
	mux := http.NewServeMux()
	mux.Handle("/api/v1/plugins", routes)
	mux.Handle("/api/v1/plugins/", routes)
	mux.Handle("/api/v1/plugin-widgets", routes)
	mux.Handle("/api/v1/plugin-widgets/", routes)
	if h.dashboard != nil {
		mux.HandleFunc("GET /api/v1/dashboard", h.GetDashboard)
	}
	mux.Handle("/", base)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if _, e := uuid.Parse(id); e != nil {
			id = uuid.NewV4().String()
		}
		w.Header().Set("X-Request-ID", id)
		mux.ServeHTTP(w, r.WithContext(plugins.WithRequestID(r.Context(), id)))
	})
}
func (h Handler) pluginAuth(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Cache-Control", "no-store")
	if !credential.MatchBearer(r.Header.Get("Authorization"), h.pluginToken) || h.pluginToken == "" {
		writeJSON(w, 401, map[string]any{"code": "unauthenticated", "message": "Invalid plugin service credential.", "retryable": false, "requestId": uuid.NewV4().String()})
		return false
	}
	if h.plugins == nil {
		writePluginFailure(w, plugins.Fail("unavailable"))
		return false
	}
	return true
}
func (h Handler) GetPlugins(w http.ResponseWriter, r *http.Request) {
	if !h.pluginAuth(w, r) {
		return
	}
	descriptors, err := h.plugins.Descriptors(r.Context())
	if err != nil {
		writePluginFailure(w, err)
		return
	}
	writeJSON(w, 200, plugins.Catalog{Plugins: descriptors, Problems: h.plugins.InstallationProblems()})
}
func (h Handler) QueryPlugin(w http.ResponseWriter, r *http.Request, pluginID, connectionID, queryID string) {
	if !h.pluginAuth(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, (64<<10)+1024)
	var body struct {
		Input json.RawMessage `json:"input"`
	}
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	err := d.Decode(&body)
	if err == nil {
		trailing := d.Decode(new(any))
		if trailing != io.EOF {
			if trailing == nil {
				err = plugins.Fail("invalid_argument")
			} else {
				err = trailing
			}
		}
	}
	if err != nil || body.Input == nil {
		var max *http.MaxBytesError
		if errors.As(err, &max) {
			writePluginFailureStatus(w, plugins.Fail("invalid_argument"), 413)
		} else {
			writePluginFailure(w, plugins.Fail("invalid_argument"))
		}
		return
	}
	if len(body.Input) > 64<<10 {
		writePluginFailureStatus(w, plugins.Fail("invalid_argument"), 413)
		return
	}
	result, err := h.plugins.Query(r.Context(), pluginID, connectionID, queryID, body.Input)
	if err != nil {
		writePluginFailure(w, err)
		return
	}
	writeJSON(w, 200, result)
}
func writePluginFailure(w http.ResponseWriter, err error) {
	code := plugins.ErrorCode(err)
	statuses := map[string]int{"invalid_argument": 400, "permission_denied": 403, "not_found": 404, "conflict": 409, "precondition_failed": 412, "precondition_required": 428, "resource_exhausted": 429, "invalid_response": 502, "deadline_exceeded": 504, "unavailable": 503, "collection_failed": 503}
	writePluginFailureStatus(w, err, statuses[code])
}
func writePluginFailureStatus(w http.ResponseWriter, err error, status int) {
	safe := plugins.Fail(plugins.ErrorCode(err))
	writeJSON(w, status, struct {
		*plugins.Failure
		RequestID string `json:"requestId"`
	}{safe, w.Header().Get("X-Request-ID")})
}
