package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	adapterhttp "github.com/araihu/balemoh/internal/adapters/http"
	"github.com/araihu/balemoh/internal/api/generated"
	"github.com/araihu/balemoh/internal/application/catalog"
)

type fakeCatalog struct {
	staging    []catalog.Candidate
	homepage   []catalog.Candidate
	updated    catalog.Candidate
	syncResult catalog.SyncResult
	listErr    error
	pinErr     error
	unpinErr   error
	syncErr    error
	pinnedIDs  []string
	unpinnedID string
}

type fakeSnapshotImporter struct {
	snapshot catalog.Snapshot
	result   catalog.SyncResult
	err      error
	called   bool
}

func (f *fakeSnapshotImporter) ImportSnapshot(_ context.Context, snapshot catalog.Snapshot) (catalog.SyncResult, error) {
	f.called = true
	f.snapshot = snapshot
	return f.result, f.err
}

func (f *fakeCatalog) ListStaging(context.Context) ([]catalog.Candidate, error) {
	return f.staging, f.listErr
}

func (f *fakeCatalog) ListHomepage(context.Context) ([]catalog.Candidate, error) {
	return f.homepage, f.listErr
}

func (f *fakeCatalog) Pin(_ context.Context, id string) (catalog.Candidate, error) {
	f.pinnedIDs = append(f.pinnedIDs, id)
	return f.updated, f.pinErr
}

func (f *fakeCatalog) Unpin(_ context.Context, id string) (catalog.Candidate, error) {
	f.unpinnedID = id
	return f.updated, f.unpinErr
}

func (f *fakeCatalog) Sync(context.Context) (catalog.SyncResult, error) {
	return f.syncResult, f.syncErr
}

func httpTestCandidate(pinned bool) catalog.Candidate {
	candidate := catalog.NewCandidate(
		catalog.SourceRef{Kind: "kubernetes", ID: "cluster-1"},
		catalog.ResourceRef{Kind: "httproute", Namespace: "apps", Name: "grafana"},
		time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC),
	)
	candidate.Description = "Grafana dashboard"
	candidate.Metadata = map[string]string{"team": "platform"}
	candidate.Images = []string{"grafana/grafana:11"}
	candidate.Endpoints = []catalog.Endpoint{{
		Name:       "web",
		URL:        "https://grafana.example.test/",
		Port:       443,
		Protocol:   "https",
		Provenance: "kubernetes.httproute",
	}}
	if pinned {
		value := time.Date(2026, 8, 17, 12, 30, 0, 0, time.UTC)
		candidate.PinnedAt = &value
	}
	return candidate
}

func catalogHandler(catalogService *fakeCatalog) http.Handler {
	return generated.HandlerFromMux(adapterhttp.NewHandler(fakeChecker{}, catalogService), http.NewServeMux())
}

func TestCatalogListRoutesProjectCandidates(t *testing.T) {
	candidate := httpTestCandidate(true)
	service := &fakeCatalog{staging: []catalog.Candidate{candidate}, homepage: []catalog.Candidate{candidate}}
	handler := catalogHandler(service)

	tests := []struct {
		name string
		path string
	}{
		{name: "staging", path: "/api/v1/staging/services"},
		{name: "homepage", path: "/api/v1/homepage/services"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
			}
			if got := recorder.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("Content-Type = %q, want application/json", got)
			}
			var response generated.ServiceListResponse
			if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if len(response.Services) != 1 || response.Services[0].Id != candidate.ID {
				t.Fatalf("services = %#v, want candidate %q", response.Services, candidate.ID)
			}
			if response.Services[0].Source.Kind != "kubernetes" || response.Services[0].Resource.Namespace == nil || *response.Services[0].Resource.Namespace != "apps" {
				t.Fatalf("source/resource projection = %#v/%#v", response.Services[0].Source, response.Services[0].Resource)
			}
			if response.Services[0].PinnedAt == nil || response.Services[0].Endpoints[0].Url != "https://grafana.example.test/" {
				t.Fatalf("candidate projection = %#v", response.Services[0])
			}
			if len(response.Services[0].Images) != 1 || response.Services[0].Images[0] != "grafana/grafana:11" {
				t.Fatalf("images projection = %#v", response.Services[0].Images)
			}
		})
	}
}

func TestCatalogPinAndUnpinRoutes(t *testing.T) {
	candidate := httpTestCandidate(true)
	service := &fakeCatalog{updated: candidate}
	handler := catalogHandler(service)

	pinRecorder := httptest.NewRecorder()
	handler.ServeHTTP(pinRecorder, httptest.NewRequest(http.MethodPost, "/api/v1/staging/services/abc/pin", nil))
	if pinRecorder.Code != http.StatusOK {
		t.Fatalf("pin status = %d, want %d", pinRecorder.Code, http.StatusOK)
	}
	if len(service.pinnedIDs) != 1 || service.pinnedIDs[0] != "abc" {
		t.Fatalf("pinned IDs = %#v, want [abc]", service.pinnedIDs)
	}

	unpinRecorder := httptest.NewRecorder()
	handler.ServeHTTP(unpinRecorder, httptest.NewRequest(http.MethodDelete, "/api/v1/staging/services/abc/pin", nil))
	if unpinRecorder.Code != http.StatusOK {
		t.Fatalf("unpin status = %d, want %d", unpinRecorder.Code, http.StatusOK)
	}
	if service.unpinnedID != "abc" {
		t.Fatalf("unpinned ID = %q, want abc", service.unpinnedID)
	}
}

func TestCatalogSyncRouteReturnsCounts(t *testing.T) {
	service := &fakeCatalog{syncResult: catalog.SyncResult{Sources: 2, Candidates: 3}}
	recorder := httptest.NewRecorder()
	catalogHandler(service).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/discovery/sync", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var response generated.DiscoverySyncResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Sources != 2 || response.Candidates != 3 {
		t.Fatalf("response = %#v, want sources=2 candidates=3", response)
	}
}

func TestCatalogErrorsAreSanitized(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		method     string
		service    fakeCatalog
		wantStatus int
		wantCode   string
	}{
		{
			name:       "missing pin",
			path:       "/api/v1/staging/services/missing/pin",
			method:     http.MethodPost,
			service:    fakeCatalog{pinErr: catalog.ErrNotFound},
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "catalog failure",
			path:       "/api/v1/staging/services",
			method:     http.MethodGet,
			service:    fakeCatalog{listErr: errors.New("database password leaked")},
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   "unavailable",
		},
		{
			name:       "sync failure",
			path:       "/api/v1/discovery/sync",
			method:     http.MethodPost,
			service:    fakeCatalog{syncErr: errors.New("docker socket leaked")},
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   "unavailable",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			catalogHandler(&test.service).ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
			if strings.Contains(recorder.Body.String(), "leaked") {
				t.Fatalf("body = %q, contains internal error", recorder.Body.String())
			}
			var response generated.ErrorResponse
			if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if response.Code != test.wantCode {
				t.Fatalf("error code = %q, want %q", response.Code, test.wantCode)
			}
		})
	}
}

func TestFederationImportRequiresAuthenticationAndRegistration(t *testing.T) {
	catalogService := &fakeCatalog{}
	importer := &fakeSnapshotImporter{}
	handler := generated.HandlerFromMux(
		adapterhttp.NewHandlerWithFederation(fakeChecker{}, catalogService, importer, map[catalog.SourceRef]string{{Kind: "container", ID: "docker-local"}: "gateway-secret"}),
		http.NewServeMux(),
	)
	payload := federationPayload(httpTestCandidate(false))
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodPost, "/api/v1/federation/snapshots", strings.NewReader(string(body))))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want %d", unauthorized.Code, http.StatusUnauthorized)
	}

	forbiddenPayload := payload
	forbiddenPayload.Source.Id = "other-host"
	forbiddenPayload.Candidates[0].Source.Id = "other-host"
	forbiddenPayload.Candidates[0].Id = catalog.StableID(
		catalog.SourceRef{Kind: "kubernetes", ID: "other-host"},
		catalog.ResourceRef{Kind: "httproute", Namespace: "apps", Name: "grafana"},
	)
	forbiddenBody, err := json.Marshal(forbiddenPayload)
	if err != nil {
		t.Fatalf("marshal forbidden payload: %v", err)
	}
	forbidden := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/federation/snapshots", strings.NewReader(string(forbiddenBody)))
	request.Header.Set("Authorization", "Bearer gateway-secret")
	handler.ServeHTTP(forbidden, request)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("forbidden status = %d, want %d", forbidden.Code, http.StatusForbidden)
	}
}

func TestFederationImportStoresSnapshotAndDoesNotImportPins(t *testing.T) {
	catalogService := &fakeCatalog{}
	importer := &fakeSnapshotImporter{result: catalog.SyncResult{Sources: 1, Candidates: 1}}
	handler := generated.HandlerFromMux(
		adapterhttp.NewHandlerWithFederation(fakeChecker{}, catalogService, importer, map[catalog.SourceRef]string{{Kind: "kubernetes", ID: "cluster-1"}: "gateway-secret"}),
		http.NewServeMux(),
	)
	payload := federationPayload(httpTestCandidate(true))
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/federation/snapshots", strings.NewReader(string(body)))
	request.Header.Set("Authorization", "Bearer gateway-secret")
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if importer.snapshot.Source != (catalog.SourceRef{Kind: "kubernetes", ID: "cluster-1"}) || len(importer.snapshot.Candidates) != 1 {
		t.Fatalf("imported snapshot = %#v, want source and one candidate", importer.snapshot)
	}
	if importer.snapshot.Candidates[0].PinnedAt != nil {
		t.Fatal("imported candidate has remote pin state")
	}
	var response generated.DiscoverySyncResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Sources != 1 || response.Candidates != 1 {
		t.Fatalf("response = %#v, want sources=1 candidates=1", response)
	}
}

func TestFederationImportRejectsOmittedCandidates(t *testing.T) {
	importer := &fakeSnapshotImporter{}
	handler := generated.HandlerFromMux(
		adapterhttp.NewHandlerWithFederation(fakeChecker{}, &fakeCatalog{}, importer, map[catalog.SourceRef]string{{Kind: "kubernetes", ID: "cluster-1"}: "gateway-secret"}),
		http.NewServeMux(),
	)
	body, err := json.Marshal(map[string]any{
		"source":     map[string]string{"kind": "kubernetes", "id": "cluster-1"},
		"observedAt": "2026-08-17T12:00:00Z",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/federation/snapshots", strings.NewReader(string(body)))
	request.Header.Set("Authorization", "Bearer gateway-secret")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if importer.called {
		t.Fatal("importer was called for a payload without candidates")
	}
}

func TestFederationImportRejectsOversizedTrailingPayload(t *testing.T) {
	handler := generated.HandlerFromMux(
		adapterhttp.NewHandlerWithFederation(fakeChecker{}, &fakeCatalog{}, &fakeSnapshotImporter{}, map[catalog.SourceRef]string{{Kind: "kubernetes", ID: "cluster-1"}: "gateway-secret"}),
		http.NewServeMux(),
	)
	payload, err := json.Marshal(federationPayload(httpTestCandidate(false)))
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	body := append(payload, []byte(strings.Repeat(" ", (2<<20)+1))...)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/federation/snapshots", strings.NewReader(string(body)))
	request.Header.Set("Authorization", "Bearer gateway-secret")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestFederationImportBindsCredentialsToSource(t *testing.T) {
	first := catalog.SourceRef{Kind: "kubernetes", ID: "cluster-1"}
	second := catalog.SourceRef{Kind: "container", ID: "docker-local"}
	importer := &fakeSnapshotImporter{result: catalog.SyncResult{Sources: 1, Candidates: 1}}
	handler := generated.HandlerFromMux(
		adapterhttp.NewHandlerWithFederation(fakeChecker{}, &fakeCatalog{}, importer, map[catalog.SourceRef]string{
			first:  "cluster-secret",
			second: "container-secret",
		}),
		http.NewServeMux(),
	)
	candidate := catalog.NewCandidate(second, catalog.ResourceRef{Kind: "container", Name: "whoami"}, time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC))
	payload, err := json.Marshal(federationPayload(candidate))
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/federation/snapshots", strings.NewReader(string(payload)))
	request.Header.Set("Authorization", "Bearer cluster-secret")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	if importer.called {
		t.Fatal("importer was called with a credential for another source")
	}
}

func TestFederationImportDisabledReturnsNotFound(t *testing.T) {
	recorder := httptest.NewRecorder()
	payload, err := json.Marshal(federationPayload(httpTestCandidate(false)))
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/federation/snapshots", strings.NewReader(string(payload)))
	catalogHandler(&fakeCatalog{}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func federationPayload(candidate catalog.Candidate) generated.FederationSnapshot {
	resource := generated.ResourceRef{Kind: candidate.Resource.Kind, Name: candidate.Resource.Name}
	if candidate.Resource.Namespace != "" {
		namespace := candidate.Resource.Namespace
		resource.Namespace = &namespace
	}
	endpoints := make([]generated.ServiceEndpoint, 0, len(candidate.Endpoints))
	for _, endpoint := range candidate.Endpoints {
		endpoints = append(endpoints, generated.ServiceEndpoint{
			Name:       endpoint.Name,
			Url:        endpoint.URL,
			Port:       int32(endpoint.Port),
			Protocol:   endpoint.Protocol,
			Provenance: endpoint.Provenance,
		})
	}
	return generated.FederationSnapshot{
		Source: generated.SourceRef{Kind: candidate.Source.Kind, Id: candidate.Source.ID},
		Candidates: []generated.FederatedCandidate{{
			Id:          candidate.ID,
			Source:      generated.SourceRef{Kind: candidate.Source.Kind, Id: candidate.Source.ID},
			Resource:    resource,
			DisplayName: candidate.DisplayName,
			Description: candidate.Description,
			Metadata:    candidate.Metadata,
			Endpoints:   endpoints,
			Images:      candidate.Images,
			ObservedAt:  candidate.ObservedAt,
		}},
		ObservedAt: candidate.ObservedAt,
	}
}
