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

func TestServicePageKeepsDraftAndSaves(t *testing.T) {
	c := &fakeCatalog{staging: []api.ServiceCandidate{{Id: "one", DisplayName: "Original"}, {Id: "two", DisplayName: "Second"}}}
	handler, err := New(c, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	submit := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/staging/services/one/edit", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	c.mutationErr = errors.New("upstream private failure")
	w := submit("displayName=Draft&description=Mine&address=https%3A%2F%2Fexample.test")
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), `value="Draft"`) || strings.Contains(w.Body.String(), "upstream private failure") {
		t.Fatal(w.Body.String())
	}
	c.mutationErr = nil
	w = submit("displayName=Saved&description=Mine&address=javascript%3Aalert%281%29")
	if w.Code != http.StatusBadRequest || c.staging[0].DisplayName != "Original" {
		t.Fatal("invalid edit persisted")
	}
	w = submit("displayName=Saved&description=Mine&address=https%3A%2F%2Fexample.test")
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/staging?notice=edited" || c.staging[0].DisplayName != "Saved" {
		t.Fatalf("save: %d %s", w.Code, w.Body.String())
	}

	r := httptest.NewRequest("GET", "/staging/services/one/edit", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if !strings.Contains(w.Body.String(), `rel="canonical" href="https://balemoh.decastro.me/staging/services/one/edit"`) {
		t.Fatal("editor canonical missing")
	}
	for _, want := range []string{"Resources", "Back to Staging", `property="og:title"`, `property="og:image"`, `name="twitter:card"`} {
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("service page missing %s", want)
		}
	}
	r = httptest.NewRequest("GET", "/staging", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	for _, unwanted := range []string{">Resources</", ">Actions</", `id="service-editor-body"`} {
		if strings.Contains(w.Body.String(), unwanted) {
			t.Errorf("staging retains %s", unwanted)
		}
	}
	if !strings.Contains(w.Body.String(), `data-table-row-link="/staging/services/one/edit"`) || !strings.Contains(w.Body.String(), `name="pinned"`) {
		t.Fatal("linked rows or pin checkbox missing")
	}
}
