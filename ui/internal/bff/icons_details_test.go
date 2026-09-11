package bff

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIconDetailsFragmentAndStandalonePage(t *testing.T) {
	handler, err := New(&fakeCatalog{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for _, requestType := range []string{"", "full", "partial"} {
		fragment := requestType == "partial"
		r := httptest.NewRequest("GET", "/icons/selfhst:authgear/details", nil)
		r.Header.Set("HX-Request", "true")
		r.Header.Set("HX-Request-Type", requestType)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		body := w.Body.String()
		for _, want := range []string{"Authgear", "CC-BY-4.0", "Light variant", "Dark variant"} {
			if !strings.Contains(body, want) {
				t.Errorf("fragment=%v: missing %s", fragment, want)
			}
		}
		if w.Code != 200 {
			t.Fatalf("status %d", w.Code)
		}
		if fragment {
			if strings.Contains(body, "<html") || !strings.Contains(w.Header().Get("HX-Trigger"), "iconDetails") {
				t.Fatal("incorrect drawer fragment")
			}
		} else if !strings.Contains(body, `href="https://balemoh.decastro.me/icons/selfhst:authgear/details"`) {
			t.Fatal("missing route canonical")
		}
	}
	r := httptest.NewRequest("GET", "/icons/missing/details", nil)
	r.Header.Set("HX-Request-Type", "partial")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 503 || !strings.Contains(w.Body.String(), `role="alert"`) {
		t.Fatal("missing icon must show swappable recovery message")
	}
}
