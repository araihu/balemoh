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
		if w.Code != 200 || !strings.Contains(body, "1 icons") || strings.Contains(body, `id="icon-scroll-next"`) {
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

func TestIconScrollReturnsOnlyRemainingCards(t *testing.T) {
	handler, err := New(&fakeCatalog{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		page  string
		count int
	}{{"1", 48}, {"2", 19}, {"3", 0}} {
		r := httptest.NewRequest("GET", "/icons?source=goshtoso&page="+tc.page, nil)
		r.Header.Set("HX-Request", "true")
		r.Header.Set("HX-Target", "icon-scroll-next")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		body := w.Body.String()
		if got := strings.Count(body, `class="balemoh-icon-tile-content"`); got != tc.count {
			t.Errorf("page %s: got %d cards, want %d", tc.page, got, tc.count)
		}
		if strings.Contains(body, "<html") || strings.Contains(body, `id="icon-picker-results"`) || strings.Contains(body, "Icons by selfh.st") {
			t.Fatal("batch includes page wrapper or attribution")
		}
		if tc.page != "1" && strings.Contains(body, `id="icon-scroll-next"`) {
			t.Fatal("last batch offers another request")
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
