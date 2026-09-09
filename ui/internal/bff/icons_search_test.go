package bff

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIconSearchFragmentAndDirectNavigation(t *testing.T) {
	handler, err := New(&fakeCatalog{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []bool{false, true} {
		r := httptest.NewRequest("GET", "/icons?q=appflowy&source=selfhst&page=9", nil)
		if fragment {
			r.Header.Set("HX-Request", "true")
			r.Header.Set("HX-Target", "icon-picker-results")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		body := w.Body.String()
		if w.Code != 200 || !strings.Contains(body, "1 icons") || !strings.Contains(body, "Page 1 of 1") {
			t.Fatalf("fragment=%v: search did not filter and clamp page: status %d", fragment, w.Code)
		}
		if strings.Contains(body, `id="icon-search"`) == fragment {
			t.Fatalf("fragment=%v: incorrect search control replacement", fragment)
		}
		if !strings.Contains(body, "Uploaded icons are unavailable") {
			t.Fatal("search hid upload availability error")
		}
	}
}
