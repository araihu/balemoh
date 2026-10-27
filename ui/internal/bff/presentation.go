package bff

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	api "github.com/araihu/balemoh/client"
	"github.com/araihu/balemoh/ui/internal/view"
	"github.com/araihu/goshtoso/components/themeplayground"
	"github.com/araihu/goshtoso/themes"
)

func fontOptions(fonts []api.UploadedFont) []themes.FontOption {
	out := themes.DefaultFonts()
	for _, f := range fonts {
		out = append(out, themes.FontOption{Source: "upload", ID: f.Id, Label: f.Name + " [" + f.Id + "]", Weight: f.Weight, Style: f.Style, Variable: f.Variable, Families: []string{f.Family}, Format: f.Format, URL: "/presentation/fonts/" + url.PathEscape(f.Id) + "/file"})
	}
	return out
}
func appearanceFrom(state api.PresentationState, fonts []themes.FontOption, recovery bool) view.Appearance {
	a := view.Appearance{Theme: "goshtoso", Mode: "system", Recovery: recovery}
	if recovery {
		a.Warning = "Temporary Goshtoso/System recovery mode. Global settings change only when you save."
		return a
	}
	a.BrandIconID = pointerValue(state.BrandIconId)
	a.BrandLogoID = pointerValue(state.BrandLogoId)
	if state.Settings.Selection.Kind == "builtin" {
		if state.Settings.Selection.Key == nil {
			a.Warning = "Saved appearance could not be loaded. Using Goshtoso/System temporarily."
			return a
		}
		if _, e := themes.Preset(themes.Key(*state.Settings.Selection.Key)); e != nil {
			a.Warning = "Preset unavailable. Using Goshtoso/System temporarily."
			return a
		}
		a.Theme = *state.Settings.Selection.Key
	} else {
		css, e := themes.RenderCSS(state.Definition, func(ref themes.FontRef) (themes.ResolvedFont, error) { return themes.ResolveFont(fonts, ref) })
		if e != nil {
			a.Warning = "Saved fonts or theme could not be loaded. Using Goshtoso/System temporarily."
			return a
		}
		a.Theme = "custom"
		a.CSS = css
		for _, ref := range []themes.FontRef{state.Definition.Typography.Title, state.Definition.Typography.Body} {
			for _, font := range fonts {
				if font.Ref() == ref && font.Source != "system" {
					a.FontWatch = append(a.FontWatch, font)
					break
				}
			}
		}
	}
	a.Mode = string(state.Settings.ColorMode)
	return a
}
func (s *server) appearance(ctx context.Context, recovery bool) view.Appearance {
	fallback := view.Appearance{Theme: "goshtoso", Mode: "system", Warning: "Saved appearance is unavailable. Using Goshtoso/System temporarily."}
	if s.presentation == nil {
		return fallback
	}
	state, e := s.presentation.Presentation(ctx)
	if e != nil {
		return fallback
	}
	fonts := themes.DefaultFonts()
	if state.Settings.Selection.Kind == "custom" {
		uploaded, err := s.presentation.Fonts(ctx)
		if err != nil {
			return fallback
		}
		fonts = fontOptions(uploaded)
	}
	return appearanceFrom(state, fonts, recovery)
}
func (s *server) themesPage(ctx context.Context, recovery bool) (view.ThemesPage, view.Appearance) {
	d := view.ThemesPage{Recovery: recovery, Error: "Settings and revisions could not be loaded. Saving is unavailable."}
	fallback := view.Appearance{Theme: "goshtoso", Mode: "system", Warning: d.Error}
	if s.presentation == nil {
		return d, fallback
	}
	state, e := s.presentation.Presentation(ctx)
	if e != nil {
		return d, fallback
	}
	appearance := appearanceFrom(state, themes.DefaultFonts(), recovery)
	catalog, e := s.presentation.Themes(ctx)
	if e != nil {
		return d, appearance
	}
	uploads, e := s.presentation.Fonts(ctx)
	if e != nil {
		return d, fallback
	}
	d.Fonts = fontOptions(uploads)
	appearance = appearanceFrom(state, d.Fonts, recovery)
	d.SettingsRevision = state.Settings.Revision
	d.CatalogRevision = catalog.CatalogRevision
	d.ColorMode = string(state.Settings.ColorMode)
	d.Initial = state.Definition
	for _, t := range catalog.Builtins {
		d.Items = append(d.Items, view.ThemeItem{ID: "builtin:" + t.Key, Name: t.Label, Definition: t.Definition})
	}
	for _, t := range catalog.Custom {
		d.Items = append(d.Items, view.ThemeItem{ID: t.Id, Name: t.Name, Revision: t.Revision, Definition: t.Definition, BrandIconID: pointerValue(t.BrandIconId), BrandLogoID: pointerValue(t.BrandLogoId)})
	}
	if state.Settings.Selection.Kind == "builtin" && state.Settings.Selection.Key != nil {
		d.SelectedID = "builtin:" + *state.Settings.Selection.Key
	} else if state.Settings.Selection.Id != nil {
		d.SelectedID = *state.Settings.Selection.Id
	}
	for _, f := range uploads {
		d.Uploads = append(d.Uploads, view.FontItem{ID: f.Id, Name: f.Name, Weight: f.Weight, Style: f.Style, Variable: f.Variable, Revision: f.Revision, UsedBy: f.UsedBy})
	}
	if themes.Validate(d.Initial) != nil || d.SelectedID == "" {
		return d, fallback
	}
	d.Available = true
	d.Error = ""
	return d, appearance
}
func presentationJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *server) presentationEditor(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	d, appearance := s.themesPage(ctx, r.URL.Query().Get("appearance") == "recovery")
	if !d.Available {
		s.render(w, r, view.PageData{Title: "Theme editor", Description: "Create and edit a custom Balemoh theme.", Active: "nav-presentation", Path: r.URL.Path, ThemeEditor: &d, Appearance: appearance}, http.StatusServiceUnavailable)
		return
	}
	id := r.URL.Query().Get("themeID")
	if id == "" {
		id = d.SelectedID
	}
	found := false
	for _, item := range d.Items {
		if item.ID == id {
			d.SelectedID, d.Initial = item.ID, item.Definition
			d.Creating = r.URL.Query().Get("new") == "1" || item.Revision == ""
			found = true
			break
		}
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	title := "Edit theme"
	if d.Creating {
		title = "Create theme"
	}
	s.render(w, r, view.PageData{Title: title, Description: "Create and edit a custom Balemoh theme.", Active: "nav-presentation", Path: r.URL.Path, ThemeEditor: &d, Appearance: appearance}, http.StatusOK)
}
func bffPresentationError(w http.ResponseWriter, e error) {
	var failure *presentationFailure
	if errors.As(e, &failure) {
		presentationJSON(w, failure.status, failure.detail)
		return
	}
	presentationJSON(w, 503, api.ErrorResponse{Code: "unavailable", Message: "Operation not confirmed. Keep your draft and inspect saved state before starting another command."})
}
func (s *server) presentationAvailable(w http.ResponseWriter) bool {
	if s.presentation == nil {
		bffPresentationError(w, errors.New("unavailable"))
		return false
	}
	return true
}
func (s *server) savePresentation(w http.ResponseWriter, r *http.Request) {
	if !s.presentationAvailable(w) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 128<<10)
	var command api.PresentationSave
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if dec.Decode(&command) != nil || dec.Decode(new(any)) != io.EOF {
		presentationJSON(w, 400, api.ErrorResponse{Code: "invalid_request", Message: "Invalid draft. No settings were changed."})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	_, e := s.presentation.SavePresentation(ctx, command)
	if e != nil {
		bffPresentationError(w, e)
		return
	}
	if r.Header.Get("Accept") == "application/json" {
		presentationJSON(w, 200, map[string]string{"redirect": "/presentation#themes"})
	} else {
		redirect(w, r, "/presentation#themes")
	}
}
func (s *server) presentationPreview(w http.ResponseWriter, r *http.Request) {
	if !s.presentationAvailable(w) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	uploaded, e := s.presentation.Fonts(ctx)
	if e != nil {
		http.Error(w, "Font catalog unavailable", 503)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "form-action 'none'; base-uri 'none'")
	_ = themeplayground.Frame(themeplayground.FrameConfig{ID: "presentation-theme-preview", Title: "Balemoh preview", Content: view.ThemePreviewContent(), Head: view.ThemePreviewHead(), Fonts: fontOptions(uploaded), FontCatalogURL: "/presentation/fonts/catalog"}).Render(ctx, w)
}
func (s *server) presentationFontCatalog(w http.ResponseWriter, r *http.Request) {
	if !s.presentationAvailable(w) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	fonts, e := s.presentation.Fonts(ctx)
	if e != nil {
		bffPresentationError(w, e)
		return
	}
	presentationJSON(w, 200, fontOptions(fonts))
}
func (s *server) presentationFontFile(w http.ResponseWriter, r *http.Request) {
	if !s.presentationAvailable(w) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	b, headers, status, e := s.presentation.FontFile(ctx, r.PathValue("fontID"), r.Header.Get("If-None-Match"))
	if e != nil {
		bffPresentationError(w, e)
		return
	}
	for _, key := range []string{"Content-Type", "ETag", "Cache-Control", "X-Content-Type-Options", "Content-Security-Policy"} {
		if value := headers.Get(key); value != "" {
			w.Header().Set(key, value)
		}
	}
	w.WriteHeader(status)
	if status == 200 {
		_, _ = w.Write(b)
	}
}
func (s *server) uploadPresentationFont(w http.ResponseWriter, r *http.Request) {
	if !s.presentationAvailable(w) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	r.Body = http.MaxBytesReader(w, r.Body, (10<<20)+(64<<10))
	font, e := s.presentation.UploadFont(ctx, r.Header.Get("Content-Type"), r.Body)
	if e != nil {
		bffPresentationError(w, e)
		return
	}
	presentationJSON(w, 200, font)
}
func (s *server) createPresentationTheme(w http.ResponseWriter, r *http.Request) {
	if !s.presentationAvailable(w) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 128<<10)
	var create api.ThemeCreate
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(&create) != nil || d.Decode(new(any)) != io.EOF {
		presentationJSON(w, 400, api.ErrorResponse{Code: "invalid_request", Message: "Invalid theme."})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	t, e := s.presentation.CreateTheme(ctx, create)
	if e != nil {
		bffPresentationError(w, e)
		return
	}
	presentationJSON(w, 200, t)
}
func (s *server) managePresentationTheme(w http.ResponseWriter, r *http.Request) {
	if !s.presentationAvailable(w) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	if r.ParseForm() != nil {
		presentationJSON(w, 400, api.ErrorResponse{Code: "invalid_request", Message: "Invalid form."})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	var e error
	if r.PathValue("action") == "delete" {
		e = s.presentation.DeleteTheme(ctx, r.PathValue("themeID"), r.FormValue("expectedRevision"))
	} else if r.PathValue("action") == "rename" {
		var t api.CustomTheme
		t, e = s.presentation.RenameTheme(ctx, r.PathValue("themeID"), api.ThemeRename{Name: r.FormValue("name"), ExpectedRevision: r.FormValue("expectedRevision")})
		if e == nil {
			presentationJSON(w, 200, t)
			return
		}
	} else {
		http.NotFound(w, r)
		return
	}
	if e != nil {
		bffPresentationError(w, e)
		return
	}
	presentationJSON(w, 200, map[string]bool{"deleted": true})
}
func (s *server) deletePresentationFont(w http.ResponseWriter, r *http.Request) {
	if !s.presentationAvailable(w) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	if r.ParseForm() != nil {
		presentationJSON(w, 400, api.ErrorResponse{Code: "invalid_request", Message: "Invalid form."})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	if e := s.presentation.DeleteFont(ctx, r.PathValue("fontID"), r.FormValue("expectedRevision")); e != nil {
		bffPresentationError(w, e)
		return
	}
	presentationJSON(w, 200, map[string]bool{"deleted": true})
}
func (s *server) presentationConflict(w http.ResponseWriter, r *http.Request) {
	if !s.presentationAvailable(w) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	state, e := s.presentation.Presentation(ctx)
	if e != nil {
		bffPresentationError(w, e)
		return
	}
	var t *api.CustomTheme
	if id := strings.TrimSpace(r.URL.Query().Get("themeID")); id != "" {
		saved, err := s.presentation.Theme(ctx, id)
		if err != nil {
			bffPresentationError(w, err)
			return
		}
		t = &saved
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	b, _ := json.MarshalIndent(struct {
		Presentation api.PresentationState `json:"presentation"`
		Theme        *api.CustomTheme      `json:"theme,omitempty"`
	}{state, t}, "", "  ")
	_ = view.PresentationConflict(string(b)).Render(ctx, w)
}

func (s *server) uploadThemeBrand(w http.ResponseWriter, r *http.Request) {
	if s.icons == nil {
		presentationJSON(w, 503, api.ErrorResponse{Code: "unavailable", Message: "Image uploads unavailable."})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, (2<<20)+(32<<10))
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		presentationJSON(w, 400, api.ErrorResponse{Code: "invalid_request", Message: "Choose an image up to 2 MiB."})
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, _, err := r.FormFile("image")
	if err != nil {
		presentationJSON(w, 400, api.ErrorResponse{Code: "invalid_request", Message: "Choose a logo or icon image."})
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		presentationJSON(w, 400, api.ErrorResponse{Code: "invalid_request", Message: "Image could not be read. Reselect it to retry."})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	saved, err := s.icons.SaveIcon(ctx, "", api.IconUpload{Name: "Theme branding image", Tags: "theme-brand", Data: &data})
	if err != nil {
		presentationJSON(w, 400, api.ErrorResponse{Code: "invalid_request", Message: "Image upload was not confirmed. Use a PNG, JPEG, WebP or passive SVG up to 2 MiB. The current draft was preserved."})
		return
	}
	if !strings.HasPrefix(saved.Id, "upload:") || len(saved.Id) > 200 {
		presentationJSON(w, 503, api.ErrorResponse{Code: "unavailable", Message: "Image reference was not confirmed. Keep your draft and retry the upload."})
		return
	}
	presentationJSON(w, 200, map[string]string{"id": saved.Id, "url": view.IconImageURL(saved.Id)})
}

func (s *server) themeBrandManifest(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("iconID")
	if s.icons == nil || !strings.HasPrefix(id, "upload:") || len(id) > 200 {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	_, mime, err := s.icons.IconImage(ctx, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "no-cache")
	_ = json.NewEncoder(w).Encode(map[string]any{"icons": []map[string]string{{"src": view.IconImageURL(id), "type": mime, "purpose": "any"}}})
}
