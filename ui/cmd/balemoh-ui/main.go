package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/araihu/balemoh/ui/iconpacks/selfhst"
	"github.com/araihu/balemoh/ui/internal/bff"
	"github.com/araihu/balemoh/ui/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	options, err := config.Load()
	if err != nil {
		return err
	}

	apiClient, err := bff.NewAPIClient(options.APIBaseURL, &http.Client{Timeout: options.RequestTimeout})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	icons := startIconCache(ctx, options.IconCacheDir, options.IconStartupTimeout, selfhst.Ensure, slog.Default())
	handler, err := bff.NewWithIconHandler(apiClient, options.RequestTimeout, icons)
	if err != nil {
		return fmt.Errorf("create UI handler: %w", err)
	}

	server := &http.Server{
		Addr:              options.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: options.RequestTimeout,
	}
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve UI: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), options.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown UI: %w", err)
		}
		return nil
	}
}
