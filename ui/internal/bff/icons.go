package bff

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	api "github.com/araihu/balemoh/client"
	"github.com/araihu/balemoh/client/iconassets"
	"github.com/araihu/balemoh/ui/internal/view"
	"io"
	"io/fs"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
)

func (s *server) iconList(ctx context.Context) ([]view.LibraryIcon, error) {
	list := make([]view.LibraryIcon, 0, len(iconassets.Entries))
	for _, i := range iconassets.Entries {
		list = append(list, view.LibraryEntry(i))
	}
	if s.icons == nil {
		return list, fmt.Errorf("Uploads unavailable")
	}
	uploads, err := s.icons.Icons(ctx)
	if err != nil {
		return list, err
	}
	for _, i := range uploads {
		list = append(list, view.LibraryIcon{ID: i.Id, Name: i.Name, Tags: i.Tags, Source: "uploads", URL: view.IconImageURL(i.Id), Digest: i.Digest, UsedBy: i.UsedBy})
	}
	sort.Slice(list, func(i, j int) bool { return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name) })
	return list, nil
}
func (s *server) iconsPage(w http.ResponseWriter, r *http.Request) { s.renderIcons(w, r, "") }
func (s *server) renderIcons(w http.ResponseWriter, r *http.Request, message string) {
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	list, err := s.iconList(ctx)
	data := view.IconPage{Query: strings.TrimSpace(r.URL.Query().Get("q")), Page: 1, Picker: r.URL.Path == "/icons/picker", Error: message}
	for _, source := range []string{"goshtoso", "selfhst", "uploads"} {
		if slices.Contains(r.URL.Query()["source"], source) {
			data.Sources = append(data.Sources, source)
		}
	}
	if err != nil && data.Error == "" {
		data.Error = "Uploaded icons are unavailable. Bundled icons are still searchable."
	}
	if p, e := strconv.Atoi(r.URL.Query().Get("page")); e == nil && p > 0 {
		data.Page = p
	}
	filtered := []view.LibraryIcon{}
	for _, i := range list {
		if len(data.Sources) > 0 && !slices.Contains(data.Sources, i.Source) {
			continue
		}
		if data.Query != "" && !strings.Contains(strings.ToLower(i.Name+" "+i.ID+" "+i.Tags), strings.ToLower(data.Query)) {
			continue
		}
		filtered = append(filtered, i)
	}
	data.Total = len(filtered)
	last := max(1, (data.Total+47)/48)
	batch := r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Target") == "icon-scroll-next"
	if batch && data.Page > last {
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		return
	}
	data.Page = min(data.Page, last)
	start := (data.Page - 1) * 48
	data.Icons = filtered[start:min(start+48, len(filtered))]
	if batch {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = view.IconBatch(data).Render(r.Context(), w)
		return
	}
	if data.Picker || (r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Target") == "icon-picker-results") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = view.IconResults(data).Render(r.Context(), w)
		return
	}
	s.render(w, r, view.PageData{Title: "Icons", Description: "Search bundled icons and manage your uploaded service icons.", Path: "/icons", Active: "nav-icons", IconPage: &data}, 200)
}
func (s *server) editIcon(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	list, err := s.iconList(ctx)
	if err != nil {
		http.Error(w, "Unable to load icon", 503)
		return
	}
	for _, i := range list {
		if i.ID == r.PathValue("iconID") && i.Source == "uploads" {
			s.render(w, r, view.PageData{Title: "Edit icon", Description: "Manage this uploaded icon and its service references.", Path: view.IconEditURL(i.ID), Active: "nav-icons", IconPage: &view.IconPage{Editing: &i}}, 200)
			return
		}
	}
	http.NotFound(w, r)
}

func (s *server) iconDetails(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	list, err := s.iconList(ctx)
	var selected view.LibraryIcon
	status, message := http.StatusNotFound, "This icon is no longer available."
	if err != nil {
		status, message = http.StatusServiceUnavailable, "Unable to load this icon. Try again."
	}
	for _, i := range list {
		if i.ID == r.PathValue("iconID") {
			selected, status, message = i, http.StatusOK, ""
			break
		}
	}
	if r.Header.Get("HX-Request") != "true" {
		title := "Icon details"
		if selected.Name != "" {
			title = selected.Name + " icon"
		}
		s.render(w, r, view.PageData{Title: title, Description: "Icon source, license, and available variants in the Balemoh library.", Path: view.IconDetailsURL(r.PathValue("iconID")), Active: "nav-icons", IconPage: &view.IconPage{Details: &selected, Error: message}}, status)
		return
	}
	var body bytes.Buffer
	if err := view.IconDetails(selected, message).Render(r.Context(), &body); err != nil {
		http.Error(w, "Unable to render icon details.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Balemoh-Status", strconv.Itoa(status))
	w.Header().Set("HX-Trigger-After-Swap", `{"drawer:open":{"id":"iconDetails"}}`)
	_, _ = w.Write(body.Bytes())
}
func (s *server) uploadIcon(w http.ResponseWriter, r *http.Request) {
	if s.icons == nil {
		s.iconUploadError(w, r, view.LibraryIcon{}, "Uploads unavailable.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, (2<<20)+32768)
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		s.iconUploadError(w, r, view.LibraryIcon{}, "Choose an image up to 2 MiB.")
		return
	}
	defer r.MultipartForm.RemoveAll()
	var tagValues []string
	for index := 0; index < len(r.PostForm); index++ {
		if value := strings.TrimSpace(r.PostForm.Get(fmt.Sprintf("tags[%d]", index))); value != "" {
			tagValues = append(tagValues, value)
		}
	}
	tags := strings.Join(tagValues, ",")
	var data []byte
	file, _, err := r.FormFile("image")
	if err == nil {
		defer file.Close()
		data, err = io.ReadAll(file)
	} else if err == http.ErrMissingFile {
		err = nil
	}
	if err != nil {
		s.iconUploadError(w, r, view.LibraryIcon{Name: r.FormValue("name"), Tags: tags}, "Unable to read image. Reselect your file to retry.")
		return
	}
	id := r.PathValue("iconID")
	revision := r.FormValue("digest")
	upload := api.IconUpload{Name: r.FormValue("name"), Tags: tags, Digest: &revision}
	if len(data) > 0 {
		upload.Data = &data
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	saved, err := s.icons.SaveIcon(ctx, id, upload)
	if err != nil {
		draft := view.LibraryIcon{ID: id, Name: upload.Name, Tags: upload.Tags, Source: "uploads", Digest: revision}
		s.iconUploadError(w, r, draft, "Icon was not saved. Use a PNG, JPEG, WebP, or passive SVG up to 2 MiB. If another edit changed it, reopen the icon. Reselect your file to retry.")
		return
	}

	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Target") == "service-icon-upload-body" {
		events, _ := json.Marshal(map[string]any{
			"icon-selected": map[string]string{"id": saved.Id, "url": view.IconImageURL(saved.Id), "symbol": "", "sprite": ""},
			"modal:close":   map[string]string{"id": "service-icon-upload"},
		})
		w.Header().Set("HX-Trigger-After-Swap", string(events))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = view.ServiceIconUploadBody(view.LibraryIcon{}, "").Render(r.Context(), w)
		return
	}
	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Target") == "icon-upload-body" {
		w.Header().Set("HX-Redirect", "/icons?source=uploads")
		w.WriteHeader(http.StatusOK)
		return
	}
	redirect(w, r, view.IconEditURL(saved.Id))
}

func (s *server) iconUploadError(w http.ResponseWriter, r *http.Request, draft view.LibraryIcon, message string) {

	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Target") == "service-icon-upload-body" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = view.ServiceIconUploadBody(draft, message).Render(r.Context(), w)
		return
	}
	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Target") == "icon-upload-body" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = view.IconUploadDrawerBody(draft, message).Render(r.Context(), w)
		return
	}
	s.render(w, r, view.PageData{Title: "Edit icon", Description: "Manage uploaded service icons.", Path: "/icons", Active: "nav-icons", IconPage: &view.IconPage{Editing: &draft, Error: message}}, http.StatusOK)
}
func (s *server) deleteIcon(w http.ResponseWriter, r *http.Request) {
	if s.icons == nil {
		http.Error(w, "Icons unavailable", 503)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	if r.ParseForm() != nil {
		http.Error(w, "Invalid form", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	if err := s.icons.DeleteIcon(ctx, r.PathValue("iconID"), r.PostForm.Get("digest")); err != nil {
		s.renderIcons(w, r, "Icon could not be deleted. It may be in use or changed. Refresh, then remove service references in Staging before deleting.")
		return
	}
	redirect(w, r, "/icons?source=uploads")
}
func (s *server) iconImage(w http.ResponseWriter, r *http.Request) {
	if s.icons == nil {
		http.Error(w, "Icons unavailable", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	data, mime, err := s.icons.IconImage(ctx, r.PathValue("iconID"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(data)
}
func bundledIconsHandler() http.Handler {
	sub, err := fs.Sub(iconassets.Files, "selfhst")
	if err != nil {
		panic(err)
	}
	h := http.StripPrefix("/ui/icon-library/selfhst/", http.FileServer(http.FS(sub)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		h.ServeHTTP(w, r)
	})
}

func resolveIcon(ref string) view.LibraryIcon {
	if i, ok := iconassets.Lookup(ref); ok {
		return view.LibraryEntry(i)
	}
	if strings.HasPrefix(ref, "upload:") {
		return view.LibraryIcon{ID: ref, Source: "uploads", URL: view.IconImageURL(ref)}
	}
	return view.LibraryIcon{}
}
func serviceIcon(s api.ServiceCandidate) (string, view.LibraryIcon, view.LibraryIcon) {
	ref := pointerValue(s.Icon)
	// Use stable discovery names only; editing a service label cannot change its default.
	name := strings.ToLower(s.Resource.Name)
	aliases := map[string]string{"argo-workflows-server": "argo-workflows", "argocd-server": "argo-cd"}
	if alias := aliases[name]; alias != "" {
		name = alias
	}
	fallback := resolveIcon("selfhst:" + name)
	if fallback.ID == "" && s.Source.Kind != "kubernetes" {
		fallback = resolveIcon("goshtoso:ui-hi-16-solid-squares-2x2")
	}
	selected := fallback
	if ref != "" {
		selected = resolveIcon(ref)
	}
	return ref, selected, fallback
}
