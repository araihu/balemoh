package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/araihu/balemoh/internal/adapters/sqlite"
	"github.com/araihu/balemoh/internal/api/generated"
	"github.com/araihu/balemoh/internal/application/catalog"
	"github.com/araihu/balemoh/internal/config"
)

func TestConstructServerMigrationFailure(t *testing.T) {
	wantErr := errors.New("sentinel migration failure")
	server, db, err := constructServer(context.Background(), config.Options{
		HTTPAddr:     "127.0.0.1:0",
		DatabasePath: t.TempDir() + "/balemoh.db",
	}, func(*sql.DB) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("constructServer() error = %v, want %v", err, wantErr)
	}
	if server != nil {
		t.Fatalf("constructServer() server = %#v, want nil", server)
	}
	if db != nil {
		t.Fatalf("constructServer() db = %#v, want nil", db)
	}
}

func TestServeListenerFailure(t *testing.T) {
	wantErr := errors.New("sentinel listener failure")
	server := &http.Server{}
	if err := serve(context.Background(), server, func() error { return wantErr }); !errors.Is(err, wantErr) {
		t.Fatalf("serve() error = %v, want %v", err, wantErr)
	}
}

func TestConstructServerCatalog(t *testing.T) {
	server, db, err := constructServer(context.Background(), config.Options{
		HTTPAddr:     "127.0.0.1:0",
		DatabasePath: t.TempDir() + "/balemoh.db",
	}, sqlite.RunMigrations)
	if err != nil {
		t.Fatalf("constructServer() error = %v", err)
	}
	defer db.Close()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/staging/services", nil)
	server.Handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var response generated.ServiceListResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Services == nil {
		t.Fatal("services = nil, want empty JSON collection")
	}
	if len(response.Services) != 0 {
		t.Fatalf("services = %#v, want empty collection", response.Services)
	}
}

func TestConfiguredDiscoverersIncludesContainerSource(t *testing.T) {
	discoverers, err := configuredDiscoverers(config.Options{
		ContainerEnabled:  true,
		ContainerSourceID: "docker-local",
		ContainerHost:     "unix:///tmp/docker.sock",
	})
	if err != nil {
		t.Fatalf("configuredDiscoverers() error = %v", err)
	}
	if len(discoverers) != 1 {
		t.Fatalf("configuredDiscoverers() length = %d, want 1", len(discoverers))
	}
	if got := discoverers[0].Name(); got != "container/docker-local" {
		t.Fatalf("discoverer name = %q, want container/docker-local", got)
	}
}

type recordingSyncService struct {
	calls chan struct{}
}

func (s *recordingSyncService) Sync(context.Context) (catalog.SyncResult, error) {
	select {
	case s.calls <- struct{}{}:
	default:
	}
	return catalog.SyncResult{Sources: 1, Candidates: 1}, nil
}

func TestRunDiscoverySyncPerformsInitialAndPeriodicSync(t *testing.T) {
	service := &recordingSyncService{calls: make(chan struct{}, 4)}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		runDiscoverySync(ctx, service, 5*time.Millisecond)
		close(done)
	}()

	for call := 0; call < 2; call++ {
		select {
		case <-service.calls:
		case <-time.After(250 * time.Millisecond):
			t.Fatalf("timed out waiting for discovery sync call %d", call+1)
		}
	}

	cancel()
	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("runDiscoverySync() did not stop after context cancellation")
	}
}

func TestRunDiscoverySyncDoesNothingWhenDisabled(t *testing.T) {
	service := &recordingSyncService{calls: make(chan struct{}, 1)}
	runDiscoverySync(context.Background(), service, 0)
	select {
	case <-service.calls:
		t.Fatal("runDiscoverySync() called service with disabled interval")
	default:
	}
}
