package http

import (
	"bytes"
	"encoding/json"
	"github.com/araihu/balemoh/internal/adapters/sqlite"
	"github.com/araihu/balemoh/internal/api/generated"
	"github.com/araihu/balemoh/internal/application/dashboard"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func layoutHandler(t *testing.T) (http.Handler, *dashboard.Layouts) {
	t.Helper()
	db, e := sqlite.Open(t.Context(), t.TempDir()+"/layout.db")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	if e = sqlite.RunMigrations(db); e != nil {
		t.Fatal(e)
	}
	s := &dashboard.Layouts{Store: sqlite.NewLayoutStore(db)}
	h := Handler{}.WithDashboard(dashboard.New()).WithLayouts(s)
	return generated.HandlerWithOptions(h, generated.StdHTTPServerOptions{BaseRouter: http.NewServeMux(), ErrorHandlerFunc: RequestError}), s
}
func layoutRequest(h http.Handler, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	var raw []byte
	if text, ok := body.(string); ok {
		raw = []byte(text)
	} else if body != nil {
		raw, _ = json.Marshal(body)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(raw))
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestLayoutHTTPConditionalWritesAndStrictBodies(t *testing.T) {
	h, s := layoutHandler(t)
	get := layoutRequest(h, "GET", "/api/v1/dashboard/layout", nil, nil)
	if get.Code != 200 || get.Header().Get("ETag") != `"layout-0"` || get.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(get.Code, get.Header(), get.Body.String())
	}
	d := dashboard.DefaultLayout()
	d.Homepage.Width = "contained"
	headers := map[string]string{"If-Match": `"layout-0"`}
	for _, tc := range []struct {
		name, header string
		status       int
	}{{"missing", "", 428}, {"weak", `W/"layout-0"`, 400}, {"wildcard", "*", 400}, {"multiple", `"layout-0", "layout-1"`, 400}, {"noncanonical", `"layout-00"`, 400}, {"overflow", `"layout-9223372036854775808"`, 400}, {"stale", `"layout-1"`, 412}} {
		t.Run(tc.name, func(t *testing.T) {
			v := map[string]string{}
			if tc.header != "" {
				v["If-Match"] = tc.header
			}
			w := layoutRequest(h, "PUT", "/api/v1/dashboard/layout", d, v)
			if w.Code != tc.status || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
				t.Fatal(w.Code, w.Body.String())
			}
		})
	}
	raw, _ := json.Marshal(d)
	for _, body := range []string{strings.Replace(string(raw), `"showHeading":true,`, "", 1), strings.Replace(string(raw), `"gapPx":16`, `"gapPx":16,"unknown":1`, 1), strings.Replace(string(raw), `"gapPx":16`, `"gapPx":null`, 1), string(raw) + ` {}`} {
		w := layoutRequest(h, "PUT", "/api/v1/dashboard/layout", body, headers)
		if w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	w := layoutRequest(h, "PUT", "/api/v1/dashboard/layout", strings.Repeat(" ", 128<<10)+string(raw), headers)
	if w.Code != 413 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = layoutRequest(h, "PUT", "/api/v1/dashboard/layout", d, headers)
	if w.Code != 200 || w.Header().Get("ETag") != `"layout-1"` {
		t.Fatal(w.Code, w.Body.String())
	}
	w = layoutRequest(h, "PUT", "/api/v1/dashboard/layout", d, headers)
	if w.Code != 412 {
		t.Fatal(w.Code, w.Body.String())
	}
	state, e := s.Get(t.Context())
	if e != nil || state.Revision != "1" {
		t.Fatal(state, e)
	}
}
func TestLayoutPreviewTargetedAndReadOnlyContract(t *testing.T) {
	h, _ := layoutHandler(t)
	request := dashboard.LayoutPreviewRequest{RequestID: "11111111-1111-4111-8111-111111111111", ExpectedRevision: "0", Document: dashboard.DefaultLayout()}
	w := layoutRequest(h, "POST", "/api/v1/dashboard/layout/preview", request, nil)
	var preview dashboard.LayoutPreview
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &preview) != nil || len(preview.ResolvedGroups) != 3 || preview.Stale {
		t.Fatal(w.Code, w.Body.String())
	}
	headers := map[string]string{"If-Match": `"layout-0"`}
	order := dashboard.Ordering{Mode: "alphabetical", IDs: []string{}}
	path := "/api/v1/dashboard/layout/groups/builtin:hosts/order"
	w = layoutRequest(h, "PUT", path, order, headers)
	if w.Code != 428 {
		t.Fatal(w.Code, w.Body.String())
	}
	headers["If-Dashboard-Membership-Match"] = `"` + preview.MembershipTokens["builtin:hosts"] + `"`
	w = layoutRequest(h, "PUT", path, order, headers)
	if w.Code != 200 || w.Header().Get("ETag") != `"layout-0"` {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, path := range []string{"/api/v1/dashboard/layout", "/api/v1/dashboard/layout/order", "/api/v1/dashboard/layout/groups/builtin:hosts/order", "/api/v1/dashboard/layout/items/host-a/placement"} {
		w = layoutRequest(ReadOnly(h), "PUT", path, request.Document, headers)
		if w.Code != 403 {
			t.Fatal(path, w.Code)
		}
	}
	w = layoutRequest(ReadOnly(h), "POST", "/api/v1/dashboard/layout/preview", request, nil)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
	for _, query := range []string{"?layoutView=unknown", "?layoutView=default&layoutView=default"} {
		w = layoutRequest(h, "GET", "/api/v1/dashboard"+query, nil, nil)
		if w.Code != 400 {
			t.Fatal(query, w.Code, w.Body.String())
		}
	}
	w = layoutRequest(h, "GET", "/api/v1/dashboard?layoutView=default", nil, nil)
	if w.Code != 200 || strings.Contains(w.Body.String(), "layoutRevision") {
		t.Fatal(w.Code, w.Body.String())
	}
}
