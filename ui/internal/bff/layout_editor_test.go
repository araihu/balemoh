package bff

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
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
			_ = json.NewEncoder(w).Encode(api.LayoutPreview{RequestId: request.RequestId, DraftGeneration: 0, LayoutRevision: "1", Document: doc, ResolvedGroups: []api.LayoutResolvedGroup{{GroupId: "builtin:hosts", Title: "Hosts", DisplayLabel: "Hosts", Members: []api.LayoutMember{}}}, MembershipTokens: map[string]string{}, Stale: stale})
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
