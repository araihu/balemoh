package http_test

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateManualServiceRoute(t *testing.T) {
	handler := catalogHandler(&fakeCatalog{})
	valid := `{"requestId":"0123456789abcdef0123456789abcdef","displayName":"Router","description":"Local","address":"https://router.example.test/"}`
	for _, tc := range []struct {
		body   string
		status int
	}{
		{valid, 201}, {valid + ` {}`, 400}, {strings.Replace(valid, `"address":`, `"unknown":true,"address":`, 1), 400},
		{strings.Replace(valid, "https://router.example.test/", "javascript:alert(1)", 1), 400}, {`{}`, 400},
	} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/staging/services", strings.NewReader(tc.body)))
		if w.Code != tc.status {
			t.Fatalf("status %d want %d: %s", w.Code, tc.status, w.Body.String())
		}
		if w.Code == 201 && (!strings.Contains(w.Body.String(), `"kind":"manual"`) || !strings.HasPrefix(w.Header().Get("Location"), "/api/v1/staging/services/")) {
			t.Fatal("missing manual identity or location", w.Body.String())
		}
	}
}
