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

func TestIconSourceFiltersCombineAndSurvivePagination(t *testing.T) {
	handler, err := New(&fakeCatalog{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ query, count string }{
		{"source=goshtoso", "67 icons"},
		{"source=uploads", "0 icons"},
		{"source=goshtoso&source=uploads", "67 icons"},
		{"source=goshtoso&source=selfhst", "2960 icons"},
		{"source=goshtoso&source=goshtoso", "67 icons"},
		{"", "2960 icons"},
	} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/icons?"+tc.query, nil))
		if !strings.Contains(w.Body.String(), tc.count) {
			t.Errorf("%s: expected %s", tc.query, tc.count)
		}
		if tc.query == "source=goshtoso&source=uploads" && !strings.Contains(w.Body.String(), "source=goshtoso&amp;source=uploads") {
			t.Fatal("pagination lost selected sources")
		}
	}
}
