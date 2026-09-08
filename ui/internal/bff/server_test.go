package bff

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/araihu/balemoh/client"
)

type fakeCatalog struct {
	homepage    []client.ServiceCandidate
	staging     []client.ServiceCandidate
	homepageErr error
	stagingErr  error
	mutationErr error
	failPinID   string
	syncErr     error
	pinnedIDs   []string
	unpinnedIDs []string
	syncCalls   int
}

func (f *fakeCatalog) Homepage(context.Context) ([]client.ServiceCandidate, error) {
	return f.homepage, f.homepageErr
}

func (f *fakeCatalog) Staging(context.Context) ([]client.ServiceCandidate, error) {
	return f.staging, f.stagingErr
}

func (f *fakeCatalog) Pin(_ context.Context, id string) error {
	f.pinnedIDs = append(f.pinnedIDs, id)
	if f.mutationErr != nil {
		return f.mutationErr
	}
	if id == f.failPinID {
		return errors.New("pin failed")
	}
	for i := range f.staging {
		if f.staging[i].Id == id {
			f.staging[i].Pinned = true
		}
	}
	return nil
}

func (f *fakeCatalog) Unpin(_ context.Context, id string) error {
	f.unpinnedIDs = append(f.unpinnedIDs, id)
	return f.mutationErr
}

func (f *fakeCatalog) Sync(context.Context) (client.DiscoverySyncResponse, error) {
	f.syncCalls++
	return client.DiscoverySyncResponse{Candidates: 1, Sources: 1}, f.syncErr
}

func TestHandlerRendersHomepageAndStagingAction(t *testing.T) {
	t.Parallel()

	catalog := &fakeCatalog{
		homepage: []client.ServiceCandidate{{
			Id:          "svc-home",
			DisplayName: "Homepage service",
			Pinned:      true,
			Source:      client.SourceRef{Kind: "kubernetes", Id: "cluster-a"},
			Resource:    client.ResourceRef{Kind: "HTTPRoute", Name: "homepage", Namespace: stringPtr("apps")},
		}},
		staging: []client.ServiceCandidate{{
			Id:          "svc-stage",
			DisplayName: "Staging service",
			Images:      []string{"ghcr.io/example/app:v1"},
			Source:      client.SourceRef{Kind: "docker", Id: "raspi"},
			Resource:    client.ResourceRef{Kind: "Service", Name: "staging", Namespace: stringPtr("apps")},
		}},
	}
	handler, err := New(catalog, 2*time.Second)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	home := request(t, handler, http.MethodGet, "/", "")
	if home.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", home.Code)
	}
	for _, want := range []string{"Balemoh", "Homepage service", "Staging"} {
		if !strings.Contains(home.Body.String(), want) {
			t.Errorf("GET / body missing %q", want)
		}
	}

	staging := request(t, handler, http.MethodGet, "/staging", "")
	if staging.Code != http.StatusOK {
		t.Fatalf("GET /staging status = %d, want 200", staging.Code)
	}
	for _, want := range []string{"Staging service", "ghcr.io/example/app:v1", `action="/staging/pin"`, `name="serviceID" value="svc-stage"`, `aria-label="Sync discovery"`} {
		if !strings.Contains(staging.Body.String(), want) {
			t.Errorf("GET /staging body missing %q", want)
		}
	}
	for _, unwanted := range []string{`>Status</`, `>Pin to homepage<`, `<code>svc-stage</code>`} {
		if strings.Contains(staging.Body.String(), unwanted) {
			t.Errorf("staging contains removed UI %q", unwanted)
		}
	}
}

func TestPinSelectionValidatesBeforeMutationAndAllowsReplay(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		calls      int
	}{
		{"empty", "", http.StatusBadRequest, 0},
		{"unknown", "serviceID=svc-1&serviceID=missing", http.StatusConflict, 0},
		{"deduplicated", "serviceID=svc-1&serviceID=svc-1&serviceID=svc-2", http.StatusSeeOther, 2},
		{"oversized", "serviceID=" + strings.Repeat("x", 65536), http.StatusBadRequest, 0},
		{"too many", strings.Repeat("serviceID=svc-1&", 501), http.StatusBadRequest, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			catalog := &fakeCatalog{staging: []client.ServiceCandidate{{Id: "svc-1"}, {Id: "svc-2"}}}
			handler, err := New(catalog, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			response := request(t, handler, http.MethodPost, "/staging/pin", tc.body)
			if response.Code != tc.status || len(catalog.pinnedIDs) != tc.calls {
				t.Fatalf("status=%d calls=%v", response.Code, catalog.pinnedIDs)
			}
			if tc.status == http.StatusSeeOther {
				if response.Header().Get("Location") != "/staging?notice=selection-pinned" {
					t.Fatal("missing PRG destination")
				}
				response = request(t, handler, http.MethodPost, "/staging/pin", tc.body)
				if response.Code != http.StatusSeeOther || len(catalog.pinnedIDs) != tc.calls {
					t.Fatal("replay pinned services twice")
				}
			}
		})
	}
}

func TestPinSelectionPreservesUnfinishedItemsOnFailure(t *testing.T) {
	catalog := &fakeCatalog{staging: []client.ServiceCandidate{{Id: "svc-1"}, {Id: "svc-2"}}, failPinID: "svc-2"}
	handler, err := New(catalog, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	response := request(t, handler, http.MethodPost, "/staging/pin", "serviceID=svc-1&serviceID=svc-2")
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", response.Code)
	}
	if !strings.Contains(response.Body.String(), `name="serviceID" value="svc-2" checked`) || !strings.Contains(response.Body.String(), "Pinning stopped") {
		t.Fatal("remaining selection or error missing")
	}
	catalog.failPinID = ""
	response = request(t, handler, http.MethodPost, "/staging/pin", "serviceID=svc-1&serviceID=svc-2")
	if response.Code != http.StatusSeeOther || strings.Join(catalog.pinnedIDs, ",") != "svc-1,svc-2,svc-2" {
		t.Fatalf("retry status=%d calls=%v", response.Code, catalog.pinnedIDs)
	}
}

func TestHandlerUsesPRGForPinUnpinAndSync(t *testing.T) {
	t.Parallel()

	catalog := &fakeCatalog{}
	handler, err := New(catalog, time.Second)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	for _, tc := range []struct {
		method string
		path   string
		want   string
	}{
		{method: http.MethodPost, path: "/staging/services/svc-1/pin", want: "/staging?notice=pinned"},
		{method: http.MethodPost, path: "/staging/services/svc-1/unpin", want: "/staging?notice=unpinned"},
		{method: http.MethodPost, path: "/staging/sync", want: "/staging?notice=synced"},
	} {
		response := request(t, handler, tc.method, tc.path, "")
		if response.Code != http.StatusSeeOther {
			t.Errorf("%s %s status = %d, want 303", tc.method, tc.path, response.Code)
		}
		if got := response.Header().Get("Location"); got != tc.want {
			t.Errorf("%s %s Location = %q, want %q", tc.method, tc.path, got, tc.want)
		}
	}

	if strings.Join(catalog.pinnedIDs, ",") != "svc-1" {
		t.Errorf("pinned IDs = %#v", catalog.pinnedIDs)
	}
	if strings.Join(catalog.unpinnedIDs, ",") != "svc-1" {
		t.Errorf("unpinned IDs = %#v", catalog.unpinnedIDs)
	}
	if catalog.syncCalls != 1 {
		t.Errorf("sync calls = %d, want 1", catalog.syncCalls)
	}
}

func TestHandlerDoesNotLeakUpstreamErrors(t *testing.T) {
	t.Parallel()

	catalog := &fakeCatalog{homepageErr: errors.New("backend token=super-secret")}
	handler, err := New(catalog, time.Second)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	response := request(t, handler, http.MethodGet, "/", "")
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET / status = %d, want 503", response.Code)
	}
	body := response.Body.String()
	if !strings.Contains(body, "Catálogo indisponível") {
		t.Errorf("error page missing safe message")
	}
	if strings.Contains(body, "super-secret") {
		t.Errorf("error page leaked upstream details")
	}
}

func TestHandlerServesGoshtosoAndConsoleShellAssets(t *testing.T) {
	t.Parallel()

	handler, err := New(&fakeCatalog{}, time.Second)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	for _, tc := range []struct {
		path string
		want string
	}{
		{path: "/ui/balemoh.css", want: ".balemoh-content"},
		{path: "/consoleshell/assets/shell.css", want: "console-shell"},
		{path: "/assets/styles.css", want: "--color-"},
		{path: "/ui/icons/sprite.svg", want: `id="kubernetes-kubernetes"`},
	} {
		response := request(t, handler, http.MethodGet, tc.path, "")
		if response.Code != http.StatusOK {
			t.Errorf("GET %s status = %d, want 200", tc.path, response.Code)
			continue
		}
		if !strings.Contains(response.Body.String(), tc.want) {
			t.Errorf("GET %s body missing %q", tc.path, tc.want)
		}
	}
}

func TestAPIClientUsesGeneratedClientAndHidesResponseBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/homepage/services" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"services":[{"id":"svc-1","displayName":"Generated service","pinned":true}]}`)
	}))
	defer server.Close()

	catalog, err := NewAPIClient(server.URL, nil)
	if err != nil {
		t.Fatalf("NewAPIClient() error = %v", err)
	}
	services, err := catalog.Homepage(context.Background())
	if err != nil {
		t.Fatalf("Homepage() error = %v", err)
	}
	if len(services) != 1 || services[0].Id != "svc-1" {
		t.Fatalf("Homepage() services = %#v", services)
	}

	serverWithError := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"code":"backend","message":"token=secret"}`)
	}))
	defer serverWithError.Close()
	errorCatalog, err := NewAPIClient(serverWithError.URL, nil)
	if err != nil {
		t.Fatalf("NewAPIClient(error server) error = %v", err)
	}
	if _, err := errorCatalog.Homepage(context.Background()); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("Homepage() error = %v, want safe upstream error", err)
	}
}

func TestHandlerRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	if _, err := New(nil, time.Second); err == nil {
		t.Error("New(nil, ...) returned nil error")
	}
	if _, err := New(&fakeCatalog{}, 0); err == nil {
		t.Error("New(..., 0) returned nil error")
	}
}

func request(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func stringPtr(value string) *string { return &value }
