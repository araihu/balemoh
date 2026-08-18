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

	containeradapter "github.com/araihu/balemoh/internal/adapters/container"
	federationadapter "github.com/araihu/balemoh/internal/adapters/federation"
	httpadapter "github.com/araihu/balemoh/internal/adapters/http"
	kubernetesadapter "github.com/araihu/balemoh/internal/adapters/kubernetes"
	"github.com/araihu/balemoh/internal/adapters/sqlite"
	"github.com/araihu/balemoh/internal/api/generated"
	"github.com/araihu/balemoh/internal/application/catalog"
	"github.com/araihu/balemoh/internal/application/health"
	"github.com/araihu/balemoh/internal/config"
	"github.com/araihu/balemoh/internal/storage/sqlc"
	"k8s.io/client-go/rest"
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
	discoverers, err := configuredDiscoverers(options)
	if err != nil {
		return err
	}

	server, db, err := constructServer(context.Background(), options, sqlite.RunMigrations, discoverers...)
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return serve(ctx, server, server.ListenAndServe)
}

func constructServer(ctx context.Context, options config.Options, migrate func(*sql.DB) error, discoverers ...catalog.Discoverer) (*http.Server, *sql.DB, error) {
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
	healthService := health.NewService(sqlite.NewPinger(queries))
	publisher, err := configuredPublisher(options)
	if err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	catalogService := catalog.NewServiceWithPublisher(sqlite.NewCatalogStore(db), publisher, discoverers...)
	handler := httpadapter.NewHandlerWithFederation(
		healthService,
		catalogService,
		catalogService,
		options.FederationIngestToken,
		options.FederationAllowedSources,
	)
	httpHandler := generated.HandlerFromMux(handler, http.NewServeMux())
	return &http.Server{Addr: options.HTTPAddr, Handler: httpHandler}, db, nil
}

func configuredPublisher(options config.Options) (catalog.SnapshotPublisher, error) {
	if options.FederationGatewayURL == "" {
		return nil, nil
	}
	publisher, err := federationadapter.NewPublisher(options.FederationGatewayURL, options.FederationToken)
	if err != nil {
		return nil, fmt.Errorf("configure federation publisher: %w", err)
	}
	return publisher, nil
}

func configuredDiscoverers(options config.Options) ([]catalog.Discoverer, error) {
	discoverers := make([]catalog.Discoverer, 0, 2)
	if options.KubernetesEnabled {
		restConfig, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("load in-cluster Kubernetes config: %w", err)
		}
		discoverer, err := kubernetesadapter.NewDiscovererFromConfig(
			restConfig,
			options.KubernetesSourceID,
			options.KubernetesNamespace,
		)
		if err != nil {
			return nil, fmt.Errorf("configure Kubernetes discoverer: %w", err)
		}
		discoverers = append(discoverers, discoverer)
	}
	if options.ContainerEnabled {
		discoverer, err := containeradapter.NewDiscovererFromConfig(
			options.ContainerHost,
			options.ContainerSourceID,
		)
		if err != nil {
			return nil, fmt.Errorf("configure container discoverer: %w", err)
		}
		discoverers = append(discoverers, discoverer)
	}
	return discoverers, nil
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
