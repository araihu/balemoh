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
	"strings"
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

	server, db, catalogService, err := constructServerWithCatalog(context.Background(), options, sqlite.RunMigrations, discoverers...)
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go runDiscoverySync(ctx, catalogService, options.DiscoverySyncInterval)

	return serve(ctx, server, server.ListenAndServe)
}

func constructServer(ctx context.Context, options config.Options, migrate func(*sql.DB) error, discoverers ...catalog.Discoverer) (*http.Server, *sql.DB, error) {
	server, db, _, err := constructServerWithCatalog(ctx, options, migrate, discoverers...)
	return server, db, err
}

func constructServerWithCatalog(ctx context.Context, options config.Options, migrate func(*sql.DB) error, discoverers ...catalog.Discoverer) (*http.Server, *sql.DB, *catalog.Service, error) {
	if err := os.MkdirAll(filepath.Dir(options.DatabasePath), 0o755); err != nil {
		return nil, nil, nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sqlite.Open(ctx, options.DatabasePath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open database: %w", err)
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, nil, nil, fmt.Errorf("run migrations: %w", err)
	}

	queries := sqlc.New(db)
	healthService := health.NewService(sqlite.NewPinger(queries))
	publisher, err := configuredPublisher(options)
	if err != nil {
		_ = db.Close()
		return nil, nil, nil, err
	}
	sourceTokens, err := configuredFederationSourceTokens(options)
	if err != nil {
		_ = db.Close()
		return nil, nil, nil, fmt.Errorf("configure federation source tokens: %w", err)
	}
	catalogService := catalog.NewServiceWithPublisher(sqlite.NewCatalogStore(db), publisher, discoverers...)
	handler := httpadapter.NewHandlerWithFederation(
		healthService,
		catalogService,
		catalogService,
		sourceTokens,
	)
	httpHandler := generated.HandlerFromMux(handler, http.NewServeMux())
	return &http.Server{Addr: options.HTTPAddr, Handler: httpHandler}, db, catalogService, nil
}

type discoverySyncer interface {
	Sync(context.Context) (catalog.SyncResult, error)
}

const discoverySyncTimeout = 30 * time.Second

func runDiscoverySync(ctx context.Context, syncer discoverySyncer, interval time.Duration) {
	if syncer == nil || interval <= 0 {
		return
	}

	syncOnce := func() {
		if ctx.Err() != nil {
			return
		}
		syncContext, cancel := context.WithTimeout(ctx, discoverySyncTimeout)
		defer cancel()
		result, err := syncer.Sync(syncContext)
		if err != nil {
			log.Printf("discovery sync failed: %v", err)
			return
		}
		log.Printf("discovery sync complete: sources=%d candidates=%d", result.Sources, result.Candidates)
	}

	syncOnce()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			syncOnce()
		}
	}
}

func configuredPublisher(options config.Options) (catalog.SnapshotPublisher, error) {
	if options.FederationGatewayURL == "" {
		return nil, nil
	}
	publisher, err := federationadapter.NewPublisherWithOptions(options.FederationGatewayURL, options.FederationToken, options.FederationAllowInsecureHTTP)
	if err != nil {
		return nil, fmt.Errorf("configure federation publisher: %w", err)
	}
	return publisher, nil
}

func configuredFederationSourceTokens(options config.Options) (map[catalog.SourceRef]string, error) {
	tokens := make(map[catalog.SourceRef]string, len(options.FederationSourceTokens))
	for _, raw := range options.FederationSourceTokens {
		parts := strings.SplitN(strings.TrimSpace(raw), "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("source token must use kind/id=token format")
		}
		sourceParts := strings.SplitN(strings.TrimSpace(parts[0]), "/", 2)
		if len(sourceParts) != 2 || strings.TrimSpace(sourceParts[0]) == "" || strings.TrimSpace(sourceParts[1]) == "" || strings.TrimSpace(parts[1]) == "" {
			return nil, fmt.Errorf("source token must use kind/id=token format")
		}
		source := catalog.SourceRef{Kind: strings.TrimSpace(sourceParts[0]), ID: strings.TrimSpace(sourceParts[1])}
		if _, exists := tokens[source]; exists {
			return nil, fmt.Errorf("source tokens must not contain duplicates")
		}
		tokens[source] = strings.TrimSpace(parts[1])
	}
	return tokens, nil
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
