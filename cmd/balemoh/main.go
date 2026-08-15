package main

import (
	"context"
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

	if err := os.MkdirAll(filepath.Dir(options.DatabasePath), 0o755); err != nil {
		return fmt.Errorf("create database directory: %w", err)
	}

	db, err := sqlite.Open(context.Background(), options.DatabasePath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := sqlite.RunMigrations(db); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	queries := sqlc.New(db)
	service := health.NewService(sqlite.NewPinger(queries))
	handler := httpadapter.NewHandler(service)
	httpHandler := generated.HandlerFromMux(handler, http.NewServeMux())
	server := &http.Server{Addr: options.HTTPAddr, Handler: httpHandler}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
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
