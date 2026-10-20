package bff

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	api "github.com/araihu/balemoh/client"
)

type manualCatalog struct {
	fakeCatalog
	calls []api.ManualServiceCreate
}

func (c *manualCatalog) CreateManual(ctx context.Context, v api.ManualServiceCreate) (api.ServiceCandidate, error) {
	c.calls = append(c.calls, v)
	return c.fakeCatalog.CreateManual(ctx, v)
}

func TestManualFormPreservesDraftAndRetryKey(t *testing.T) {
	c := &manualCatalog{}
	handler, err := New(c, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/staging/services/new", nil))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	body := w.Body.String()
	keyMatch := regexp.MustCompile(`name="requestId" value="([a-f0-9]{32})"`).FindStringSubmatch(body)
	if len(keyMatch) != 2 {
		t.Fatal("missing retry key")
	}
	for _, unwanted := range []string{"Leave blank to use discovered", ">Resources<", ">Service visibility<"} {
		if strings.Contains(body, unwanted) {
			t.Fatal("discovery-only UI in manual form", unwanted)
		}
	}
	for _, want := range []string{`action="/staging/services"`, ">Add service<", ">Choose icon<"} {
		if !strings.Contains(body, want) {
			t.Fatal("missing create UI", want)
		}
	}
	values := url.Values{"requestId": {keyMatch[1]}, "displayName": {"Router draft"}, "description": {"Local admin"}, "address": {"javascript:alert(1)"}, "icon": {""}}
	submit := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/staging/services", strings.NewReader(values.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	w = submit()
	if w.Code != 400 || len(c.calls) != 0 || !strings.Contains(w.Body.String(), keyMatch[1]) {
		t.Fatal("invalid draft/key lost", w.Code)
	}
	values.Set("address", "http://router.example.test/")
	c.mutationErr = errors.New("private upstream failure")
	w = submit()
	if w.Code != 503 || !strings.Contains(w.Body.String(), `value="Router draft"`) || !strings.Contains(w.Body.String(), keyMatch[1]) || strings.Contains(w.Body.String(), "private upstream failure") {
		t.Fatal("failed create lost draft/key", w.Body.String())
	}
	c.mutationErr = nil
	w = submit()
	if w.Code != 303 || w.Header().Get("Location") != "/staging?notice=created#select-manual-created" || len(c.calls) != 2 || c.calls[0].RequestId != c.calls[1].RequestId {
		t.Fatal("retry changed identity", w.Code, c.calls)
	}
}

func TestAPIClientCreatesManualService(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/v1/staging/services" {
			t.Error("wrong request", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"created","displayName":"Router","source":{"kind":"manual","id":"local"}}`))
	}))
	defer upstream.Close()
	c, err := NewAPIClient(upstream.URL, upstream.Client())
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.CreateManual(context.Background(), api.ManualServiceCreate{RequestId: "0123456789abcdef0123456789abcdef", DisplayName: "Router", Address: "https://router.example.test/"})
	if err != nil || got.Id != "created" {
		t.Fatal(got, err)
	}
}
