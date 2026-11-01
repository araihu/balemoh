package http

import (
	"errors"
	"github.com/araihu/balemoh/internal/api/generated"
	"github.com/araihu/balemoh/internal/application/dashboard"
	"io"
	"net/http"
	"regexp"
	"strings"
)

func (h Handler) WithLayouts(service *dashboard.Layouts) Handler {
	h.layouts = service
	if h.dashboard != nil {
		h.dashboard.WithLayouts(service)
	}
	return h
}
func (h Handler) layoutReady(w http.ResponseWriter) bool {
	w.Header().Set("Cache-Control", "no-store")
	if h.layouts == nil {
		writeLayoutError(w, dashboard.LayoutFailure(503, "unavailable", "Layout service is unavailable."))
		return false
	}
	return true
}
func writeLayoutError(w http.ResponseWriter, e error) {
	w.Header().Set("Cache-Control", "no-store")
	var v *dashboard.LayoutError
	if !errors.As(e, &v) {
		v = dashboard.LayoutFailure(503, "unavailable", "Layout service is unavailable.")
	}
	writeJSON(w, v.Status, v)
}
func layoutResult(w http.ResponseWriter, state dashboard.LayoutState, e error) {
	if e != nil {
		writeLayoutError(w, e)
		return
	}
	w.Header().Set("ETag", `"layout-`+state.Revision+`"`)
	writeJSON(w, 200, state)
}
func layoutDecode(w http.ResponseWriter, r *http.Request, target any) bool {
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 128<<10))
	if e != nil {
		var max *http.MaxBytesError
		if errors.As(e, &max) {
			writeLayoutError(w, dashboard.LayoutFailure(413, "request_too_large", "Request exceeds 128 KiB."))
		} else {
			writeLayoutError(w, dashboard.LayoutFailure(400, "invalid_request", "Unable to read request."))
		}
		return false
	}
	if e = dashboard.DecodeLayoutJSON(raw, target); e != nil {
		writeLayoutError(w, e)
		return false
	}
	return true
}

var membershipHeader = regexp.MustCompile(`^"members-[a-f0-9]{64}"$`)

func layoutPreconditions(r *http.Request, membership bool) (string, string, error) {
	values := r.Header.Values("If-Match")
	if len(values) == 0 {
		return "", "", dashboard.LayoutFailure(428, "precondition_required", "If-Match is required.")
	}
	value := values[0]
	revision := strings.TrimSuffix(strings.TrimPrefix(value, `"layout-`), `"`)
	if len(values) != 1 || value != `"layout-`+revision+`"` || !dashboard.ValidLayoutRevision(revision) {
		return "", "", dashboard.LayoutFailure(400, "invalid_request", "Use one exact strong layout ETag.")
	}
	token := ""
	if membership {
		values = r.Header.Values("If-Dashboard-Membership-Match")
		if len(values) == 0 {
			return "", "", dashboard.LayoutFailure(428, "precondition_required", "If-Dashboard-Membership-Match is required.")
		}
		if len(values) != 1 || !membershipHeader.MatchString(values[0]) {
			return "", "", dashboard.LayoutFailure(400, "invalid_request", "Use one exact quoted membership token.")
		}
		token = strings.Trim(values[0], `"`)
	}
	return revision, token, nil
}
func (h Handler) GetDashboardLayout(w http.ResponseWriter, r *http.Request) {
	if !h.layoutReady(w) {
		return
	}
	v, e := h.layouts.Get(r.Context())
	layoutResult(w, v, e)
}
func (h Handler) PutDashboardLayout(w http.ResponseWriter, r *http.Request, _ generated.PutDashboardLayoutParams) {
	if !h.layoutReady(w) {
		return
	}
	revision, _, e := layoutPreconditions(r, false)
	if e != nil {
		writeLayoutError(w, e)
		return
	}
	var d dashboard.LayoutDocument
	if !layoutDecode(w, r, &d) {
		return
	}
	v, e := h.layouts.Replace(r.Context(), revision, d)
	layoutResult(w, v, e)
}
func (h Handler) PreviewDashboardLayout(w http.ResponseWriter, r *http.Request) {
	if !h.layoutReady(w) {
		return
	}
	var d dashboard.LayoutPreviewRequest
	if !layoutDecode(w, r, &d) {
		return
	}
	v, e := h.layouts.Preview(r.Context(), d)
	if e != nil {
		writeLayoutError(w, e)
		return
	}
	writeJSON(w, 200, v)
}
func (h Handler) OrderDashboardGroups(w http.ResponseWriter, r *http.Request, _ generated.OrderDashboardGroupsParams) {
	if !h.layoutReady(w) {
		return
	}
	revision, _, e := layoutPreconditions(r, false)
	if e != nil {
		writeLayoutError(w, e)
		return
	}
	var d dashboard.Ordering
	if !layoutDecode(w, r, &d) {
		return
	}
	v, e := h.layouts.OrderGroups(r.Context(), revision, d)
	layoutResult(w, v, e)
}
func (h Handler) OrderDashboardItems(w http.ResponseWriter, r *http.Request, group string, _ generated.OrderDashboardItemsParams) {
	if !h.layoutReady(w) {
		return
	}
	revision, token, e := layoutPreconditions(r, true)
	if e != nil {
		writeLayoutError(w, e)
		return
	}
	var d dashboard.Ordering
	if !layoutDecode(w, r, &d) {
		return
	}
	v, e := h.layouts.OrderItems(r.Context(), revision, group, token, d)
	layoutResult(w, v, e)
}
func (h Handler) MoveDashboardItem(w http.ResponseWriter, r *http.Request, item string, _ generated.MoveDashboardItemParams) {
	if !h.layoutReady(w) {
		return
	}
	revision, _, e := layoutPreconditions(r, false)
	if e != nil {
		writeLayoutError(w, e)
		return
	}
	var d dashboard.LayoutMove
	if !layoutDecode(w, r, &d) {
		return
	}
	v, e := h.layouts.Move(r.Context(), revision, item, d)
	layoutResult(w, v, e)
}
