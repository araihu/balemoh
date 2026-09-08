package http

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/araihu/balemoh/internal/api/generated"
	"github.com/araihu/balemoh/internal/application/icons"
	"io"
	"net/http"
)

type IconStore interface {
	List(context.Context) ([]icons.Icon, error)
	Get(context.Context, string) (icons.Icon, error)
	Save(context.Context, string, string, string, string, []byte) (icons.Icon, error)
	Delete(context.Context, string, string) error
}

func (h Handler) WithIcons(store IconStore) Handler { h.icons = store; return h }
func (h Handler) ListIcons(w http.ResponseWriter, r *http.Request) {
	if h.icons == nil {
		http.Error(w, "Icons unavailable", 503)
		return
	}
	list, err := h.icons.List(r.Context())
	if err != nil {
		iconError(w, err)
		return
	}
	result := make([]generated.UploadedIcon, 0, len(list))
	for _, i := range list {
		result = append(result, apiIcon(i))
	}
	writeJSON(w, 200, result)
}
func (h Handler) UploadIcon(w http.ResponseWriter, r *http.Request)            { h.saveIcon(w, r, "") }
func (h Handler) UpdateIcon(w http.ResponseWriter, r *http.Request, id string) { h.saveIcon(w, r, id) }
func (h Handler) saveIcon(w http.ResponseWriter, r *http.Request, id string) {
	if h.icons == nil {
		http.Error(w, "Icons unavailable", 503)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 3<<20)
	var v generated.IconUpload
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(&v) != nil || d.Decode(new(any)) != io.EOF {
		iconError(w, icons.ErrInvalid)
		return
	}
	var data []byte
	if v.Data != nil {
		data = *v.Data
	}
	revision := ""
	if v.Digest != nil {
		revision = *v.Digest
	}
	i, err := h.icons.Save(r.Context(), id, v.Name, v.Tags, revision, data)
	if err != nil {
		iconError(w, err)
		return
	}
	status := 200
	if id == "" {
		status = 201
	}
	writeJSON(w, status, apiIcon(i))
}
func (h Handler) DeleteIcon(w http.ResponseWriter, r *http.Request, id string, p generated.DeleteIconParams) {
	if h.icons == nil {
		http.Error(w, "Icons unavailable", 503)
		return
	}
	if err := h.icons.Delete(r.Context(), id, p.Digest); err != nil {
		iconError(w, err)
		return
	}
	w.WriteHeader(204)
}
func (h Handler) GetIconImage(w http.ResponseWriter, r *http.Request, id string) {
	if h.icons == nil {
		http.Error(w, "Icons unavailable", 503)
		return
	}
	i, err := h.icons.Get(r.Context(), id)
	if err != nil {
		iconError(w, err)
		return
	}
	w.Header().Set("Content-Type", i.MIME)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("ETag", `"`+i.Digest+`"`)
	if r.Header.Get("If-None-Match") == `"`+i.Digest+`"` {
		w.WriteHeader(304)
		return
	}
	_, _ = w.Write(i.Data)
}
func apiIcon(i icons.Icon) generated.UploadedIcon {
	return generated.UploadedIcon{Id: i.ID, Name: i.Name, Tags: i.Tags, Mime: i.MIME, Digest: i.Digest, UsedBy: i.UsedBy}
}
func iconError(w http.ResponseWriter, err error) {
	status := 503
	if errors.Is(err, icons.ErrInvalid) {
		status = 400
	}
	if errors.Is(err, icons.ErrNotFound) {
		status = 404
	}
	if errors.Is(err, icons.ErrConflict) {
		status = 409
	}
	http.Error(w, http.StatusText(status), status)
}
