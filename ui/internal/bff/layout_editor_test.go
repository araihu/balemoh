package bff

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	api "github.com/araihu/balemoh/client"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLayoutBFFPreservesStrictRequestAndDeniesCrossOrigin(t *testing.T) {
	var received []byte
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		received, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(422)
		_, _ = w.Write([]byte(`{"code":"invalid_layout","message":"Fix spacing.","fieldErrors":{"/document/groups/0/layout/gapPx":"Choose supported spacing."}}`))
	}))
	t.Cleanup(upstream.Close)
	client, e := NewAPIClient(upstream.URL, upstream.Client())
	if e != nil {
		t.Fatal(e)
	}
	h, e := New(client, time.Second)
	if e != nil {
		t.Fatal(e)
	}
	raw := `{"requestId":"11111111-1111-4111-8111-111111111111","draftGeneration":3,"expectedRevision":"0","document":{"unknown":null}}`
	body, _ := json.Marshal(map[string]any{"request": json.RawMessage(raw), "selection": map[string]string{"groupId": "builtin:hosts", "itemId": ""}})
	request := httptest.NewRequest("POST", "/settings/homepage/layout/preview", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, request)
	if w.Code != 422 || string(received) != raw || !strings.Contains(w.Body.String(), "/document/groups/0/layout/gapPx") || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code, string(received), w.Body.String())
	}
	request = httptest.NewRequest("POST", "/settings/homepage/layout/preview", bytes.NewReader(body))
	request.Header.Set("Origin", "https://attacker.example")
	request.Header.Set("Sec-Fetch-Site", "cross-site")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, request)
	if w.Code != 403 || calls.Load() != 1 {
		t.Fatal(w.Code, calls.Load())
	}
	for _, path := range []string{"/settings/homepage/layout", "/settings/homepage/layout/saved"} {
		request = httptest.NewRequest("GET", path, nil)
		w = httptest.NewRecorder()
		PublicReadOnly(h, "").ServeHTTP(w, request)
		if w.Code != 403 || calls.Load() != 1 {
			t.Fatal(path, w.Code, calls.Load())
		}
	}
}
func TestLayoutBFFAmbiguousCommitReturnsUncertainAndReadback(t *testing.T) {
	raw, e := os.ReadFile("../../../docs/specs/spec-001-homepage-layouts/example-layout.json")
	if e != nil {
		t.Fatal(e)
	}
	var document any
	if e = json.Unmarshal(raw, &document); e != nil {
		t.Fatal(e)
	}
	var writes atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			writes.Add(1)
			if r.Header.Get("If-Match") != `"layout-0"` {
				t.Errorf("revision lost: %s", r.Header.Get("If-Match"))
			}
			connection, _, e := http.NewResponseController(w).Hijack()
			if e != nil {
				t.Error(e)
				return
			}
			connection.Close()
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"revision": "1", "document": document})
	}))
	t.Cleanup(upstream.Close)
	client, e := NewAPIClient(upstream.URL, upstream.Client())
	if e != nil {
		t.Fatal(e)
	}
	h, e := New(client, time.Second)
	if e != nil {
		t.Fatal(e)
	}
	body, _ := json.Marshal(map[string]any{"expectedRevision": "0", "document": document})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/settings/homepage/layout", bytes.NewReader(body)))
	if w.Code != 503 || !strings.Contains(w.Body.String(), `"uncertain":true`) || writes.Load() != 1 {
		t.Fatal(w.Code, w.Body.String(), writes.Load())
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/settings/homepage/layout/saved", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"revision":"1"`) || writes.Load() != 1 {
		t.Fatal(w.Code, w.Body.String(), writes.Load())
	}
}

func TestLayoutSSEReconcilesRevisionAndPreservesContentOnlyUpdates(t *testing.T) {
	raw, e := os.ReadFile("../../../docs/specs/spec-001-homepage-layouts/example-layout.json")
	if e != nil {
		t.Fatal(e)
	}
	var doc api.LayoutDocument
	if e = json.Unmarshal(raw, &doc); e != nil {
		t.Fatal(e)
	}
	revision := "0"
	state := api.LayoutState{Revision: revision, Document: doc}
	groups := []api.DashboardGroup{{GroupId: "builtin:hosts", Title: "Hosts", DisplayLabel: "Hosts", ItemIds: []string{"host-a", "host-b"}}}
	snapshot := api.DashboardSnapshot{LayoutRevision: &revision, Groups: &groups, Items: []api.DashboardWidget{hostWidget("a", 0), hostWidget("b", 10)}, Unavailable: []string{}}
	var mu sync.Mutex
	var configReads atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path == "/api/v1/dashboard/layout" {
			configReads.Add(1)
			_ = json.NewEncoder(w).Encode(state)
		} else {
			_ = json.NewEncoder(w).Encode(snapshot)
		}
	}))
	t.Cleanup(upstream.Close)
	client, e := NewAPIClient(upstream.URL, upstream.Client())
	if e != nil {
		t.Fatal(e)
	}
	s := &server{catalog: client, timeout: time.Second, widgets: widgetRenderers()}
	done := make(chan struct{})
	streams := atomic.Int32{}
	ui := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		streams.Add(1)
		defer close(done)
		s.streamDashboard(w, r, 10*time.Millisecond)
	}))
	t.Cleanup(ui.Close)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	request, _ := http.NewRequestWithContext(ctx, "GET", ui.URL, nil)
	response, e := ui.Client().Do(request)
	if e != nil {
		t.Fatal(e)
	}
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	until := func(want string) string {
		t.Helper()
		var frame strings.Builder
		for {
			line, e := reader.ReadString('\n')
			if e != nil {
				t.Fatal(e)
			}
			if line == "\n" {
				text := frame.String()
				if strings.Contains(text, want) {
					return text
				}
				frame.Reset()
			} else {
				frame.WriteString(line)
			}
		}
	}
	until("0.0%")
	mu.Lock()
	revision = "1"
	state.Revision = "1"
	groups[0].ItemIds = []string{"host-b", "host-a"}
	state.Document.Groups[0].Ordering = api.LayoutOrdering{Mode: "custom", Ids: []string{"host-b", "host-a"}}
	state.Document.Groups[0].Placements = []api.LayoutGroupPlacement{{ItemId: "host-a"}, {ItemId: "host-b"}}
	mu.Unlock()
	structural := until(`id="dashboard-grid"`)
	if strings.Index(structural, `id="widget-host-b"`) > strings.Index(structural, `id="widget-host-a"`) {
		t.Fatal("revision/group order did not reconcile", structural)
	}
	mu.Lock()
	snapshot.Items[0] = hostWidget("a", 25)
	mu.Unlock()
	content := until("25.0%")
	if strings.Contains(content, `id="dashboard-grid"`) || strings.Contains(content, `id="widget-host-b"`) {
		t.Fatal("value update replaced geometry or neighbour", content)
	}
	if streams.Load() != 1 || configReads.Load() != 2 {
		t.Fatal("extra stream or configuration reads", streams.Load(), configReads.Load())
	}
	response.Body.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SSE cancellation leaked")
	}
}

func TestLayoutEditorRetriesStaleBootstrapBeforeEnablingDraft(t *testing.T) {
	raw, e := os.ReadFile("../../../docs/specs/spec-001-homepage-layouts/example-layout.json")
	if e != nil {
		t.Fatal(e)
	}
	var doc api.LayoutDocument
	if e = json.Unmarshal(raw, &doc); e != nil {
		t.Fatal(e)
	}
	var reads, previews atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/dashboard/layout":
			revision := "0"
			if reads.Add(1) > 1 {
				revision = "1"
			}
			_ = json.NewEncoder(w).Encode(api.LayoutState{Revision: revision, Document: doc})
		case "/api/v1/dashboard/layout/preview":
			var request api.LayoutPreviewRequest
			_ = json.NewDecoder(r.Body).Decode(&request)
			stale := previews.Add(1) == 1
			_ = json.NewEncoder(w).Encode(emptyLayoutPreview(request, "1", stale))
		case "/api/v1/dashboard":
			_ = json.NewEncoder(w).Encode(api.DashboardSnapshot{Items: []api.DashboardWidget{}, Unavailable: []string{}})
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(upstream.Close)
	client, e := NewAPIClient(upstream.URL, upstream.Client())
	if e != nil {
		t.Fatal(e)
	}
	h, e := New(client, time.Second)
	if e != nil {
		t.Fatal(e)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/settings/homepage/layout", nil))
	match := regexp.MustCompile(`data-layout-editor="([^"]*)"`).FindStringSubmatch(w.Body.String())
	if w.Code != 200 || len(match) != 2 {
		t.Fatal(w.Code, w.Body.String())
	}
	var bootstrap struct {
		State   api.LayoutState   `json:"state"`
		Preview api.LayoutPreview `json:"preview"`
	}
	if e = json.Unmarshal([]byte(html.UnescapeString(match[1])), &bootstrap); e != nil {
		t.Fatal(e)
	}
	if reads.Load() != 2 || previews.Load() != 2 || bootstrap.State.Revision != "1" || bootstrap.Preview.Stale || bootstrap.Preview.LayoutRevision != "1" {
		t.Fatal("stale bootstrap escaped bounded retry", reads.Load(), previews.Load(), bootstrap)
	}
}

func TestLayoutComparisonCoversDraftSavedAndStableMembershipWithoutWrites(t *testing.T) {
	raw, e := os.ReadFile("../../../docs/specs/spec-001-homepage-layouts/example-layout.json")
	if e != nil {
		t.Fatal(e)
	}
	var saved, draft api.LayoutDocument
	if e = json.Unmarshal(raw, &saved); e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(raw, &draft); e != nil {
		t.Fatal(e)
	}
	draft.Homepage.Width = "contained"
	draft.SourceGroups.Other = "builtin:hosts"
	draft.Groups[0].Placements = []api.LayoutGroupPlacement{{ItemId: "host-test"}}
	draft.Groups[0].InitiallyCollapsed = true
	var writes atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			writes.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.LayoutState{Revision: "4", Document: saved})
	}))
	t.Cleanup(upstream.Close)
	client, e := NewAPIClient(upstream.URL, upstream.Client())
	if e != nil {
		t.Fatal(e)
	}
	h, e := New(client, time.Second)
	if e != nil {
		t.Fatal(e)
	}
	body, _ := json.Marshal(map[string]any{"document": draft})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/settings/homepage/layout/compare", bytes.NewReader(body)))
	var response struct {
		Revision string `json:"revision"`
		HTML     string `json:"html"`
	}
	if e = json.Unmarshal(w.Body.Bytes(), &response); e != nil {
		t.Fatal(e)
	}
	if w.Code != 200 || response.Revision != "4" || writes.Load() != 0 {
		t.Fatal(w.Code, w.Body.String(), writes.Load())
	}
	for _, want := range []string{"Your draft", "Saved version", "contained", "full", "Default groups / Other items", "builtin:hosts", "host-test", "Start collapsed", "Spacing", "All layout settings and identities"} {
		if !strings.Contains(response.HTML, want) {
			t.Errorf("comparison missing %q", want)
		}
	}
}

func TestLayoutSaveRejectsUnconfirmedResponsesAndAllowsReadback(t *testing.T) {
	raw, err := os.ReadFile("../../../docs/specs/spec-001-homepage-layouts/example-layout.json")
	if err != nil {
		t.Fatal(err)
	}
	var document any
	if err = json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	state := func(revision any, doc any) string {
		b, _ := json.Marshal(map[string]any{"revision": revision, "document": doc})
		return string(b)
	}
	cases := []struct {
		name      string
		status    int
		body      string
		uncertain bool
	}{
		{"empty", 200, `{}`, true}, {"null", 200, `null`, true},
		{"missing revision", 200, `{"document":` + string(raw) + `}`, true},
		{"missing document", 200, `{"revision":"1"}`, true},
		{"empty revision", 200, state("", document), true}, {"noncanonical revision", 200, state("01", document), true},
		{"overflow revision", 200, state("9223372036854775808", document), true},
		{"implausible revision", 200, state("2", document), true},
		{"null document", 200, state("1", nil), true}, {"incomplete document", 200, state("1", map[string]any{}), true},
		{"proxy timeout", 408, `Request timeout`, true},
		{"malformed client error", 400, `<html>Bad request</html>`, true},
		{"unknown conflict", 409, `{"code":"proxy_error","message":"Proxy failed."}`, true},
		{"unknown error shape", 400, `{"code":"invalid_request","message":"Proxy failed.","gateway":true}`, true},
		{"documented validation", 422, `{"code":"invalid_layout","message":"Fix draft.","fieldErrors":{"/document/homepage/layout/columns":"Choose 1 through 4."}}`, false},
		{"bad gateway", 502, `Bad gateway`, true}, {"gateway timeout", 504, `{}`, true},
		{"intermediary unavailable", 503, `<html>Unavailable</html>`, true},
		{"unknown server error", 500, `{"code":"other","message":"Failure"}`, true},
		{"documented no write", 503, `{"code":"unavailable","message":"Storage unavailable."}`, false},
		{"valid changed", 200, state("1", document), false}, {"valid no op", 200, state("0", document), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var writes atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == "PUT" {
					writes.Add(1)
					w.WriteHeader(tc.status)
					_, _ = io.WriteString(w, tc.body)
					return
				}
				_, _ = io.WriteString(w, state("1", document))
			}))
			t.Cleanup(upstream.Close)
			client, e := NewAPIClient(upstream.URL, upstream.Client())
			if e != nil {
				t.Fatal(e)
			}
			handler, e := New(client, time.Second)
			if e != nil {
				t.Fatal(e)
			}
			body, _ := json.Marshal(map[string]any{"expectedRevision": "0", "document": document})
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest("POST", "/settings/homepage/layout", bytes.NewReader(body)))
			if tc.uncertain {
				if w.Code != 503 || !strings.Contains(w.Body.String(), `"uncertain":true`) {
					t.Fatalf("unconfirmed Save accepted: %d %s", w.Code, w.Body)
				}
			} else if strings.Contains(w.Body.String(), `"uncertain":true`) {
				t.Fatal(w.Body)
			}
			if strings.HasPrefix(tc.name, "valid") && w.Code != 200 {
				t.Fatal(w.Code, w.Body)
			}
			w = httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest("GET", "/settings/homepage/layout/saved", nil))
			if w.Code != 200 || !strings.Contains(w.Body.String(), `"revision":"1"`) || writes.Load() != 1 {
				t.Fatal(w.Code, w.Body, writes.Load())
			}
		})
	}
}

func TestLayoutStateValidationRejectsIncompleteAndInvalidDocuments(t *testing.T) {
	raw, e := os.ReadFile("../../../docs/specs/spec-001-homepage-layouts/example-layout.json")
	if e != nil {
		t.Fatal(e)
	}
	cases := map[string]func(map[string]any){
		"missing bool":     func(d map[string]any) { delete(d["groups"].([]any)[0].(map[string]any), "showHeading") },
		"null bool":        func(d map[string]any) { d["groups"].([]any)[0].(map[string]any)["showHeading"] = nil },
		"invalid geometry": func(d map[string]any) { d["homepage"].(map[string]any)["layout"].(map[string]any)["columns"] = 8 },
		"invalid defaults": func(d map[string]any) { d["sourceGroups"].(map[string]any)["host"] = "unknown" },
		"duplicate group":  func(d map[string]any) { g := d["groups"].([]any); d["groups"] = append(g, g[0]) },
		"foreign rank": func(d map[string]any) {
			d["homepage"].(map[string]any)["ordering"] = map[string]any{"mode": "custom", "ids": []string{"unknown"}}
		},
		"invalid title": func(d map[string]any) { d["groups"].([]any)[0].(map[string]any)["title"] = strings.Repeat("x", 81) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			var document map[string]any
			_ = json.Unmarshal(raw, &document)
			mutate(document)
			body, _ := json.Marshal(map[string]any{"revision": "1", "document": document})
			r := &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body))}
			_, err := layoutResponse[api.LayoutState](r, nil)
			var failure *layoutFailure
			if !errors.As(err, &failure) || !failure.uncertain {
				t.Fatal("invalid state accepted", err)
			}
		})
	}
}

func emptyLayoutPreview(request api.LayoutPreviewRequest, revision string, stale bool) api.LayoutPreview {
	p := api.LayoutPreview{RequestId: request.RequestId, DraftGeneration: request.DraftGeneration, LayoutRevision: revision, Document: request.Document, ResolvedGroups: []api.LayoutResolvedGroup{}, MembershipTokens: map[string]string{}, Stale: stale}
	for _, g := range request.Document.Groups {
		p.ResolvedGroups = append(p.ResolvedGroups, api.LayoutResolvedGroup{GroupId: g.Id, Title: g.Title, DisplayLabel: g.Title, Members: []api.LayoutMember{}})
		raw, _ := json.Marshal([]any{"layout-members-v1", g.Id, []string{}})
		p.MembershipTokens[g.Id] = fmt.Sprintf("members-%x", sha256.Sum256(raw))
	}
	return p
}
func TestLayoutPreviewRejectsMalformedSuccessBeforeRendering(t *testing.T) {
	raw, e := os.ReadFile("../../../docs/specs/spec-001-homepage-layouts/example-layout.json")
	if e != nil {
		t.Fatal(e)
	}
	var document api.LayoutDocument
	if e = json.Unmarshal(raw, &document); e != nil {
		t.Fatal(e)
	}
	var request api.LayoutPreviewRequest
	_ = json.Unmarshal([]byte(`{"requestId":"11111111-1111-4111-8111-111111111111","draftGeneration":3,"expectedRevision":"0"}`), &request)
	request.Document = document
	preview := emptyLayoutPreview(request, "0", false)
	encoded, _ := json.Marshal(preview)
	cases := map[string]func(map[string]any){
		"empty document":     func(p map[string]any) { p["document"] = map[string]any{} },
		"null document":      func(p map[string]any) { p["document"] = nil },
		"missing generation": func(p map[string]any) { delete(p, "draftGeneration") },
		"wrong generation":   func(p map[string]any) { p["draftGeneration"] = 4 },
		"wrong request":      func(p map[string]any) { p["requestId"] = "22222222-2222-4222-8222-222222222222" },
		"wrong revision":     func(p map[string]any) { p["layoutRevision"] = "01" },
		"wrong stale":        func(p map[string]any) { p["stale"] = true },
		"missing groups":     func(p map[string]any) { p["resolvedGroups"] = []any{} },
		"null members":       func(p map[string]any) { p["resolvedGroups"].([]any)[0].(map[string]any)["members"] = nil },
		"wrong token":        func(p map[string]any) { p["membershipTokens"].(map[string]any)["builtin:hosts"] = "members-fake" },
		"different draft": func(p map[string]any) {
			p["document"].(map[string]any)["homepage"].(map[string]any)["width"] = "contained"
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			var invalid map[string]any
			_ = json.Unmarshal(encoded, &invalid)
			mutate(invalid)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/dashboard/layout/preview" {
					t.Errorf("invalid preview reached rendering: %s", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(invalid)
			}))
			t.Cleanup(upstream.Close)
			client, e := NewAPIClient(upstream.URL, upstream.Client())
			if e != nil {
				t.Fatal(e)
			}
			handler, e := New(client, time.Second)
			if e != nil {
				t.Fatal(e)
			}
			body, _ := json.Marshal(map[string]any{"request": request, "selection": layoutSelection{GroupID: "builtin:hosts"}})
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest("POST", "/settings/homepage/layout/preview", bytes.NewReader(body)))
			if w.Code != 503 || strings.Contains(w.Body.String(), `"previewHtml"`) {
				t.Fatal(w.Code, w.Body)
			}
		})
	}
}

func TestLayoutPreviewPreservesHTMLSensitiveAndUnicodeMembershipIDs(t *testing.T) {
	raw, e := os.ReadFile("../../../docs/specs/spec-001-homepage-layouts/example-layout.json")
	if e != nil {
		t.Fatal(e)
	}
	var document api.LayoutDocument
	_ = json.Unmarshal(raw, &document)
	itemID := "host&<猫>"
	document.Groups[0].Placements = []api.LayoutGroupPlacement{{ItemId: itemID}}
	var request api.LayoutPreviewRequest
	_ = json.Unmarshal([]byte(`{"requestId":"11111111-1111-4111-8111-111111111111","draftGeneration":3,"expectedRevision":"0"}`), &request)
	request.Document = document
	preview := emptyLayoutPreview(request, "0", false)
	preview.ResolvedGroups[0].Members = []api.LayoutMember{{ItemId: itemID, DisplayLabel: "A host", State: "active"}}
	// Literal wire bytes pin the owning API's non-HTML-escaped token algorithm.
	token := fmt.Sprintf("members-%x", sha256.Sum256([]byte(`["layout-members-v1","builtin:hosts",["host&<猫>"]]`)))
	preview.MembershipTokens["builtin:hosts"] = token
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/dashboard/layout/preview" {
			_ = json.NewEncoder(w).Encode(preview)
		} else {
			_ = json.NewEncoder(w).Encode(api.DashboardSnapshot{Items: []api.DashboardWidget{}, Unavailable: []string{}})
		}
	}))
	t.Cleanup(upstream.Close)
	client, e := NewAPIClient(upstream.URL, upstream.Client())
	if e != nil {
		t.Fatal(e)
	}
	handler, e := New(client, time.Second)
	if e != nil {
		t.Fatal(e)
	}
	body, _ := json.Marshal(map[string]any{"request": request, "selection": layoutSelection{GroupID: "builtin:hosts", ItemID: itemID}})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("POST", "/settings/homepage/layout/preview", bytes.NewReader(body)))
	var response struct {
		Preview api.LayoutPreview `json:"preview"`
	}
	if e = json.Unmarshal(w.Body.Bytes(), &response); e != nil {
		t.Fatal(e)
	}
	if w.Code != 200 || response.Preview.ResolvedGroups[0].Members[0].ItemId != itemID || response.Preview.MembershipTokens["builtin:hosts"] != token {
		t.Fatal(w.Code, w.Body)
	}
}
