package bff

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLifecycleMutationRecovery(t *testing.T) {
	for _, tc := range []struct {
		name, action string
		err          error
		status       int
		location     string
	}{
		{"hide", "hide", nil, 303, "/staging?notice=hide&status=hidden#hidden"},
		{"show", "show", nil, 303, "/staging?notice=show&status=live#live"},
		{"purge", "purge", nil, 303, "/staging?notice=purge&status=missing#missing"},
		{"stale", "purge", &upstreamError{status: 409}, 409, ""},
		{"gone", "purge", &upstreamError{status: 404}, 404, ""},
		{"transport", "hide", errors.New("connection reset"), 503, ""},
		{"invalid", "remove", nil, 400, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, err := New(&fakeCatalog{mutationErr: tc.err}, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest("POST", "/staging/services/app/lifecycle", strings.NewReader("action="+tc.action))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != tc.status || w.Header().Get("Location") != tc.location {
				t.Fatalf("%d %v %s", w.Code, w.Header(), w.Body.String())
			}
			if tc.status >= 400 && (!strings.Contains(w.Body.String(), `role="alert"`) || !strings.Contains(w.Body.String(), "Staging")) {
				t.Fatal("error did not render in application shell")
			}
		})
	}
}
