package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/araihu/balemoh/internal/adapters/fontvalidate"
	"github.com/araihu/balemoh/internal/api/generated"
	p "github.com/araihu/balemoh/internal/application/presentation"
)

func (h Handler) WithPresentation(store p.Store) Handler { h.presentation = store; return h }
func presentationError(w http.ResponseWriter, err error) {
	var e *p.Error
	if !errors.As(err, &e) {
		e = &p.Error{Code: "unavailable", Message: "Presentation storage or catalog unavailable."}
	}
	status := map[string]int{"invalid_request": 400, "not_found": 404, "conflict": 409, "in_use": 409, "limit_exceeded": 409, "upload_too_large": 413, "unsupported_font": 415, "unavailable": 503}[e.Code]
	if status == 0 {
		status = 503
	}
	writeJSON(w, status, e)
}
func (h Handler) presentationReady(w http.ResponseWriter) bool {
	if h.presentation == nil {
		presentationError(w, errors.New("unavailable"))
		return false
	}
	w.Header().Set("Cache-Control", "no-store")
	return true
}
func presentationDecode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 128<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if dec.Decode(v) != nil || dec.Decode(new(any)) != io.EOF {
		presentationError(w, p.Invalid())
		return false
	}
	return true
}
func (h Handler) GetPresentation(w http.ResponseWriter, r *http.Request) {
	if !h.presentationReady(w) {
		return
	}
	v, e := h.presentation.State(r.Context())
	if e != nil {
		presentationError(w, e)
		return
	}
	writeJSON(w, 200, v)
}
func (h Handler) SavePresentation(w http.ResponseWriter, r *http.Request) {
	if !h.presentationReady(w) {
		return
	}
	var c p.Save
	if !presentationDecode(w, r, &c) {
		return
	}
	v, e := h.presentation.Save(r.Context(), c)
	if e != nil {
		presentationError(w, e)
		return
	}
	writeJSON(w, 200, v)
}
func (h Handler) ListThemes(w http.ResponseWriter, r *http.Request) {
	if !h.presentationReady(w) {
		return
	}
	v, e := h.presentation.Catalog(r.Context())
	if e != nil {
		presentationError(w, e)
		return
	}
	writeJSON(w, 200, v)
}
func (h Handler) GetTheme(w http.ResponseWriter, r *http.Request, id string) {
	if !h.presentationReady(w) {
		return
	}
	if !p.ValidID(id, "theme") {
		presentationError(w, p.Invalid())
		return
	}
	v, e := h.presentation.GetTheme(r.Context(), id)
	if e != nil {
		presentationError(w, e)
		return
	}
	writeJSON(w, 200, v)
}
func (h Handler) CreateTheme(w http.ResponseWriter, r *http.Request) {
	if !h.presentationReady(w) {
		return
	}
	var c p.Create
	if !presentationDecode(w, r, &c) {
		return
	}
	v, created, e := h.presentation.CreateTheme(r.Context(), c)
	if e != nil {
		presentationError(w, e)
		return
	}
	status := 200
	if created {
		status = 201
	}
	writeJSON(w, status, v)
}
func (h Handler) RenameTheme(w http.ResponseWriter, r *http.Request, id string) {
	if !h.presentationReady(w) {
		return
	}
	var c p.Rename
	if !presentationDecode(w, r, &c) {
		return
	}
	v, e := h.presentation.RenameTheme(r.Context(), id, c)
	if e != nil {
		presentationError(w, e)
		return
	}
	writeJSON(w, 200, v)
}
func (h Handler) DeleteTheme(w http.ResponseWriter, r *http.Request, id string, params generated.DeleteThemeParams) {
	if !h.presentationReady(w) {
		return
	}
	if e := h.presentation.DeleteTheme(r.Context(), id, params.ExpectedRevision); e != nil {
		presentationError(w, e)
		return
	}
	w.WriteHeader(204)
}
func (h Handler) ListFonts(w http.ResponseWriter, r *http.Request) {
	if !h.presentationReady(w) {
		return
	}
	v, e := h.presentation.Fonts(r.Context())
	if e != nil {
		presentationError(w, e)
		return
	}
	writeJSON(w, 200, v)
}
func (h Handler) UploadFont(w http.ResponseWriter, r *http.Request) {
	if !h.presentationReady(w) {
		return
	}
	if r.ContentLength > (10<<20)+(64<<10) {
		presentationError(w, &p.Error{Code: "upload_too_large", Message: "Upload exceeds multipart limit."})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, (10<<20)+(64<<10))
	reader, e := r.MultipartReader()
	if e != nil {
		presentationError(w, p.Invalid())
		return
	}
	fields := map[string]string{}
	seen := map[string]bool{}
	var data []byte
	fail := func(err error) {
		var max *http.MaxBytesError
		if errors.As(err, &max) {
			presentationError(w, &p.Error{Code: "upload_too_large", Message: "Upload exceeds multipart limit."})
		} else {
			presentationError(w, p.Invalid())
		}
	}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			fail(err)
			return
		}
		name := part.FormName()
		if seen[name] || name != "id" && name != "name" && name != "file" {
			presentationError(w, p.Invalid())
			return
		}
		seen[name] = true
		limit := int64(1024)
		if name == "file" {
			limit = p.MaxFontBytes
		}
		b, err := io.ReadAll(io.LimitReader(part, limit+1))
		if err != nil {
			fail(err)
			return
		}
		if int64(len(b)) > limit {
			presentationError(w, &p.Error{Code: "upload_too_large", Message: "File exceeds 10 MiB or metadata limit."})
			return
		}
		if name == "file" {
			data = b
		} else {
			fields[name] = string(b)
		}
		if err = part.Close(); err != nil {
			fail(err)
			return
		}
	}
	if _, err := io.Copy(io.Discard, r.Body); err != nil {
		fail(err)
		return
	}
	if !p.ValidID(fields["id"], "font") || !p.ValidName(fields["name"]) || !seen["file"] {
		presentationError(w, p.Invalid())
		return
	}
	f, e := fontvalidate.Validate(r.Context(), data)
	if e != nil {
		presentationError(w, e)
		return
	}
	f.ID, f.Name, f.Data = fields["id"], fields["name"], data
	v, created, e := h.presentation.CreateFont(r.Context(), f)
	if e != nil {
		presentationError(w, e)
		return
	}
	status := 200
	if created {
		status = 201
	}
	writeJSON(w, status, v)
}
func (h Handler) DeleteFont(w http.ResponseWriter, r *http.Request, id string, params generated.DeleteFontParams) {
	if !h.presentationReady(w) {
		return
	}
	if e := h.presentation.DeleteFont(r.Context(), id, params.ExpectedRevision); e != nil {
		presentationError(w, e)
		return
	}
	w.WriteHeader(204)
}
func (h Handler) GetFontFile(w http.ResponseWriter, r *http.Request, id string) {
	if !h.presentationReady(w) {
		return
	}
	if !p.ValidID(id, "font") {
		presentationError(w, p.Invalid())
		return
	}
	v, e := h.presentation.Font(r.Context(), id)
	if e != nil {
		presentationError(w, e)
		return
	}
	w.Header().Set("Content-Type", v.MIME)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, max-age=0, must-revalidate")
	w.Header().Set("ETag", `"`+v.Digest+`"`)
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	if strings.TrimSpace(r.Header.Get("If-None-Match")) == `"`+v.Digest+`"` {
		w.WriteHeader(304)
		return
	}
	w.WriteHeader(200)
	_, _ = w.Write(v.Data)
}

// RequestError keeps generated parameter-validation failures in the API envelope.
func RequestError(w http.ResponseWriter, r *http.Request, _ error) {
	if strings.HasPrefix(r.URL.Path, "/api/v1/presentation") || strings.HasPrefix(r.URL.Path, "/api/v1/themes") || strings.HasPrefix(r.URL.Path, "/api/v1/fonts") {
		presentationError(w, p.Invalid())
		return
	}
	http.Error(w, "Invalid request parameters", http.StatusBadRequest)
}
