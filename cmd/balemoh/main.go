package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	httpadapter "github.com/araihu/balemoh/internal/adapters/http"
	"github.com/araihu/balemoh/internal/adapters/sqlite"
	"github.com/araihu/balemoh/internal/api/generated"
	"github.com/araihu/balemoh/internal/application/health"
	"github.com/araihu/balemoh/internal/config"
	"github.com/araihu/balemoh/internal/storage/sqlc"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	options, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	server, db, err := constructServer(context.Background(), options, sqlite.RunMigrations)
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return serve(ctx, server, server.ListenAndServe)
}

func constructServer(ctx context.Context, options config.Options, migrate func(*sql.DB) error) (*http.Server, *sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(options.DatabasePath), 0o755); err != nil {
		return nil, nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sqlite.Open(ctx, options.DatabasePath)
	if err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("run migrations: %w", err)
	}

	queries := sqlc.New(db)
	service := health.NewService(sqlite.NewPinger(queries))
	handler := httpadapter.NewHandler(service)
	httpHandler := generated.HandlerFromMux(handler, http.NewServeMux())
	return &http.Server{Addr: options.HTTPAddr, Handler: httpHandler}, db, nil
}

func serve(ctx context.Context, server *http.Server, listen func() error) error {
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- listen()
	}()

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			return fmt.Errorf("shutdown HTTP server: %w", err)
		}
		return nil
	}
}
