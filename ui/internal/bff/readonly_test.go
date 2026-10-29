package bff

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

func TestPublicReadOnlyDeniesForgedActionsBeforeUpstream(t *testing.T) {
	for _, request := range []struct{ method, path string }{
		{"POST", "/staging/sync"}, {"POST", "/staging/services/a/pin"}, {"POST", "/presentation/save"}, {"POST", "/icons/upload"},
		{"POST", "/notifications/read-all"}, {"POST", "/notifications/clear"}, {"POST", "/plugins/p/connections/c/pages/action"},
		{"GET", "/staging/services/new"}, {"GET", "/staging/services/a/edit"}, {"GET", "/notifications/clear"},
		{"GET", "/dashboard/events"}, {"GET", "/hosts/a/events"},
	} {
		t.Run(request.method+request.path, func(t *testing.T) {
			h := PublicReadOnly(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("blocked request reached upstream") }))
			response := httptest.NewRecorder()
			h.ServeHTTP(response, httptest.NewRequest(request.method, request.path, nil))
			if response.Code != 403 && response.Code != 404 {
				t.Fatalf("status=%d", response.Code)
			}
		})
	}
}

func TestPublicReadOnlyRetainsBudgetUntilTimedOutWorkStops(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		started := make(chan struct{}, 16)
		release := make(chan struct{})
		h := PublicReadOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { started <- struct{}{}; <-release; w.WriteHeader(200) }))
		var callers sync.WaitGroup
		for range 16 {
			callers.Go(func() { h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil)) })
		}
		for range 16 {
			<-started
		}
		time.Sleep(9 * time.Second)
		callers.Wait()
		response := httptest.NewRecorder()
		h.ServeHTTP(response, httptest.NewRequest("GET", "/", nil))
		if response.Code != 429 {
			t.Errorf("timed-out work lost its concurrency slot: %d", response.Code)
		}
		close(release)
		synctest.Wait()
		response = httptest.NewRecorder()
		h.ServeHTTP(response, httptest.NewRequest("GET", "/", nil))
		if response.Code != 200 {
			t.Errorf("completed work did not release budget: %d", response.Code)
		}
	})
}

func TestPublicReadOnlyGlobalRateAndIndependentHealth(t *testing.T) {
	h := PublicReadOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	limited := false
	for range 100 {
		response := httptest.NewRecorder()
		h.ServeHTTP(response, httptest.NewRequest("GET", "/", nil))
		limited = limited || response.Code == 429
	}
	if !limited {
		t.Fatal("unbounded sequential request rate")
	}
	response := httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequest("GET", "/healthz", nil))
	if response.Code != 200 {
		t.Fatalf("admission caused probe failure: %d", response.Code)
	}
}

func TestPublicReadOnlyCachesOnlySuccessfulStaticResponses(t *testing.T) {
	for _, test := range []struct {
		path          string
		status        int
		cookie, cache bool
	}{
		{"/assets/styles.css", 200, false, true}, {"/ui/balemoh.css", 200, false, true},
		{"/ui/host-live.js", 200, true, false}, {"/assets/missing.css", 404, false, false},
		{"/assets/styles.css?variant=anything", 200, false, false}, {"/", 200, false, false},
	} {
		t.Run(test.path, func(t *testing.T) {
			h := PublicReadOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if test.cookie {
					w.Header().Set("Set-Cookie", "test=value")
				}
				w.WriteHeader(test.status)
			}))
			response := httptest.NewRecorder()
			h.ServeHTTP(response, httptest.NewRequest("GET", test.path, nil))
			if got := strings.HasPrefix(response.Header().Get("Cache-Control"), "public"); got != test.cache {
				t.Fatalf("cache policy %q", response.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestPublicReadOnlyBoundsConcurrentWorkWithoutVisitorState(t *testing.T) {
	started := make(chan struct{}, 16)
	release := make(chan struct{})
	h := PublicReadOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		select {
		case <-release:
		case <-r.Context().Done():
		}
		w.WriteHeader(200)
	}))
	var workers sync.WaitGroup
	for range 16 {
		workers.Go(func() { h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil)) })
	}
	for range 16 {
		<-started
	}
	response := httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequest("GET", "/", nil))
	close(release)
	workers.Wait()
	if response.Code != 429 || response.Header().Get("Retry-After") == "" {
		t.Fatalf("overload=%d", response.Code)
	}
}

func TestPublicReadOnlyPageHasNoticeAndNoStream(t *testing.T) {
	inner, err := New(&fakeCatalog{}, 1000000000)
	if err != nil {
		t.Fatal(err)
	}
	h := PublicReadOnly(inner)
	for range 2 {
		response := httptest.NewRecorder()
		h.ServeHTTP(response, httptest.NewRequest("GET", "/", nil))
		if response.Code != 200 || !strings.Contains(response.Body.String(), "Shared read-only demo") || strings.Contains(response.Body.String(), `hx-sse:connect=`) {
			t.Fatalf("demo page=%d", response.Code)
		}
	}
}
