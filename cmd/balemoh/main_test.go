package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"testing"

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
