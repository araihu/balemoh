package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/araihu/balemoh/internal/api/generated"
	"github.com/araihu/balemoh/internal/application/dashboard"
)

type dashboardProvider struct {
	enabled bool
	calls   int
}

func (p *dashboardProvider) Kind() string { return "test" }
func (p *dashboardProvider) Selected(context.Context) ([]dashboard.Widget, error) {
	return []dashboard.Widget{{SourceID: "id", Kind: "text", Title: "hello", Data: map[string]string{"text": "<script>not HTML</script>"}}}, nil
}
func (p *dashboardProvider) SetSelected(_ context.Context, _ string, value bool) error {
	p.enabled = value
	p.calls++
	return nil
}
func TestDashboardHTTPContract(t *testing.T) {
	provider := &dashboardProvider{}
	handler := generated.HandlerFromMux(Handler{}.WithDashboard(dashboard.New(provider)), http.NewServeMux())
	for _, tc := range []struct {
		body   string
		status int
	}{{`{}`, 400}, {`{"enabled":null}`, 400}, {`{"enabled":true,"extra":1}`, 400}, {`{"enabled":true} {}`, 400}, {`{"enabled":true}`, 204}, {`{"enabled":true}`, 204}, {`{"enabled":false}`, 204}} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("PUT", "/api/v1/dashboard/widgets/test/id", strings.NewReader(tc.body)))
		if w.Code != tc.status {
			t.Fatalf("%s = %d", tc.body, w.Code)
		}
	}
	if provider.enabled || provider.calls != 3 {
		t.Fatal("invalid request changed state")
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/dashboard", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"kind":"text"`) || !strings.Contains(w.Body.String(), `"presentation":"card"`) {
		t.Fatal(w.Body.String())
	}
}
