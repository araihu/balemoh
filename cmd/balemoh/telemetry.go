package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/araihu/balemoh/internal/adapters/federation"
	"github.com/araihu/balemoh/internal/adapters/sqlite"
	collector "github.com/araihu/balemoh/internal/adapters/telemetry"
	"github.com/araihu/balemoh/internal/application/telemetry"
	"github.com/araihu/balemoh/internal/config"
	"k8s.io/client-go/rest"
)

func startTelemetry(ctx context.Context, db *sql.DB, options config.Options) error {
	store := sqlite.NewTelemetryStore(db)
	collectors := []func(context.Context) telemetry.Batch{}
	if options.TelemetryEnabled {
		if options.ContainerEnabled {
			c, err := collector.NewDocker(options.ContainerHost, options.ContainerSourceID, options.HostProcPath)
			if err != nil {
				return fmt.Errorf("configure Docker telemetry: %w", err)
			}
			collectors = append(collectors, c.Collect)
		}
		if options.KubernetesEnabled {
			config, err := rest.InClusterConfig()
			if err != nil {
				return err
			}
			c, err := collector.NewKubernetes(config, options.KubernetesSourceID, options.KubernetesNamespace)
			if err != nil {
				return err
			}
			collectors = append(collectors, c.Collect)
		}
	}
	var publisher *federation.Publisher
	if len(collectors) > 0 && options.FederationGatewayURL != "" {
		var err error
		publisher, err = federation.NewPublisherWithOptions(options.FederationGatewayURL, options.FederationToken, options.FederationAllowInsecureHTTP)
		if err != nil {
			return err
		}
	}
	go func() {
		tick := time.NewTicker(time.Minute)
		defer tick.Stop()
		for {
			cycle, cancel := context.WithTimeout(ctx, 30*time.Second)
			if err := store.Prune(cycle, time.Now()); err != nil && ctx.Err() == nil {
				log.Printf("telemetry retention failed: %v", err)
			}
			cancel()
			for _, collect := range collectors {
				cycle, cancel := context.WithTimeout(ctx, 30*time.Second)
				batch := collect(cycle)
				err := store.Ingest(cycle, batch, time.Now())
				if err == nil && publisher != nil {
					err = publisher.PublishTelemetry(cycle, batch)
				}
				cancel()
				if err != nil && ctx.Err() == nil {
					log.Printf("telemetry cycle failed: %v", err)
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()
	return nil
}
