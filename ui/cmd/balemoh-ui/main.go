package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	pluginui "github.com/araihu/balemoh/plugin-sdk/ui"
	"github.com/araihu/balemoh/ui/iconpacks/selfhst"
	"github.com/araihu/balemoh/ui/internal/bff"
	"github.com/araihu/balemoh/ui/internal/config"
	"github.com/araihu/balemoh/ui/internal/plugins"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--plugin-host-info" {
		fmt.Println(`{"Protocol":"balemoh.plugins/v1alpha1","UIProfile":"balemoh.goshtoso/v1alpha1"}`)
		return
	}
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
	var registry *plugins.Registry
	if options.PluginPreviewConfig != "" {
		registry, err = plugins.Open(ctx, options.PluginPreviewConfig)
		if err != nil {
			return fmt.Errorf("load plugin previews: %w", err)
		}
		defer registry.Close()
	}
	if options.PluginConfig != "" {
		source, sourceErr := plugins.NewSource(options.APIBaseURL, options.PluginServiceTokenFile, options.RequestTimeout)
		if sourceErr != nil {
			return sourceErr
		}
		registry, err = plugins.OpenData(ctx, options.PluginConfig, source)
		if err != nil {
			return fmt.Errorf("load plugins: %w", err)
		}
		defer registry.Close()
	}
	handler, err := bff.NewWithPlugins(apiClient, options.RequestTimeout, icons, registry)
	if err != nil {
		return fmt.Errorf("create UI handler: %w", err)
	}

	inner := handler
	handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inner.ServeHTTP(w, r.WithContext(pluginui.WithDisplayContext(r.Context(), pluginui.DisplayContext{Theme: options.Theme, ColorScheme: options.ColorScheme})))
	})
	server := &http.Server{
		Addr:              options.HTTPAddr,
		Handler:           handler,
		BaseContext:       func(net.Listener) context.Context { return ctx },
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
