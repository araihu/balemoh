package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	adapterhttp "github.com/araihu/balemoh/internal/adapters/http"
	"github.com/araihu/balemoh/internal/api/generated"
)

type fakeChecker struct {
	err error
}

func (f fakeChecker) Check(context.Context) error { return f.err }

func TestHealthz(t *testing.T) {
	t.Run("healthy", func(t *testing.T) {
		handler := generated.HandlerFromMux(adapterhttp.NewHandler(fakeChecker{}), http.NewServeMux())
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/healthz", nil)

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
		if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
			t.Fatalf("Content-Type = %q, want %q", contentType, "application/json")
		}

		var response generated.HealthResponse
		if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if response.Status != generated.Ok {
			t.Fatalf("status = %q, want %q", response.Status, generated.Ok)
		}
	})

	t.Run("dependency failure", func(t *testing.T) {
		dependencyError := errors.New("database password leaked")
		handler := generated.HandlerFromMux(adapterhttp.NewHandler(fakeChecker{err: dependencyError}), http.NewServeMux())
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/healthz", nil)

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
		}
		if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
			t.Fatalf("Content-Type = %q, want %q", contentType, "application/json")
		}
		if body := recorder.Body.String(); body == "" || strings.Contains(body, dependencyError.Error()) {
			t.Fatalf("response body = %q, must be non-empty and exclude dependency error", body)
		}

		var response generated.ErrorResponse
		if err := json.NewDecoder(strings.NewReader(recorder.Body.String())).Decode(&response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if response.Code != "unavailable" || response.Message != "service unavailable" {
			t.Fatalf("response = %#v, want unavailable/service unavailable", response)
		}
	})
}
