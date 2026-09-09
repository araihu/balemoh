package bff

import (
	"errors"
	api "github.com/araihu/balemoh/client"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEditorKeepsDraftAndReturnsOnlyEditedRow(t *testing.T) {
	c := &fakeCatalog{staging: []api.ServiceCandidate{{Id: "one", DisplayName: "Original"}, {Id: "two", DisplayName: "Second"}}}
	handler, err := New(c, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	submit := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/staging/services/one/edit", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	c.mutationErr = errors.New("upstream private failure")
	w := submit("displayName=Draft&description=Mine&address=https%3A%2F%2Fexample.test")
	if w.Header().Get("X-Balemoh-Status") != "503" || !strings.Contains(w.Body.String(), `value="Draft"`) || strings.Contains(w.Body.String(), "upstream private failure") {
		t.Fatal(w.Body.String())
	}
	c.mutationErr = nil
	w = submit("displayName=Saved&description=Mine&address=javascript%3Aalert%281%29")
	if w.Header().Get("X-Balemoh-Status") != "400" || c.staging[0].DisplayName != "Original" {
		t.Fatal("invalid edit persisted")
	}
	w = submit("displayName=Saved&description=Mine&address=https%3A%2F%2Fexample.test")
	if !strings.Contains(w.Body.String(), `hx-swap-oob="outerHTML"`) || !strings.Contains(w.Body.String(), "service-row-one") || strings.Contains(w.Body.String(), "service-row-two") || strings.Contains(w.Body.String(), "<html") || c.staging[0].DisplayName != "Saved" {
		t.Fatal(w.Body.String())
	}
	r := httptest.NewRequest("GET", "/staging/services/one/edit", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if !strings.Contains(w.Body.String(), `rel="canonical" href="https://balemoh.decastro.me/staging/services/one/edit"`) {
		t.Fatal("editor canonical missing")
	}
}
