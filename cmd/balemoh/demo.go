package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/araihu/balemoh/internal/adapters/sqlite"
	"github.com/araihu/balemoh/internal/application/catalog"
	"github.com/araihu/balemoh/internal/application/notifications"
	"github.com/araihu/balemoh/internal/config"
	"github.com/araihu/balemoh/internal/demo"
)

func startDemo(ctx context.Context, db *sql.DB, catalogService *catalog.Service, notificationService *notifications.Service, options config.Options) (func(), error) {
	telemetry := sqlite.NewTelemetryStore(db)
	seedCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	err := demo.Seed(seedCtx, catalogService, telemetry, sqlite.NewDashboardSelections(db), time.Now().UTC())
	cancel()
	if err != nil {
		return nil, fmt.Errorf("seed synthetic demo: %w", err)
	}
	now := time.Now().UTC().Truncate(24 * time.Hour)
	for i, title := range []string{"Synthetic demo is ready", "Demo service restarted", "Demo memory pressure"} {
		severity := notifications.SeverityInfo
		if i > 0 {
			severity = notifications.SeverityWarning
		}
		_, err := notificationService.Publish(ctx, notifications.Source{Kind: notifications.SourceCore, ID: "demo"}, notifications.Message{EventID: fmt.Sprintf("demo-%s-%d", now.Format("2006-01-02"), i), Severity: severity, Title: title, Body: "Fictional data. Shared read state is disabled in this demo.", OccurredAt: now, Destination: notifications.Destination{Kind: "core", Route: "/hosts"}})
		if err != nil {
			return nil, fmt.Errorf("seed demo notifications: %w", err)
		}
	}
	listener, err := net.Listen("tcp", options.DemoAlertmanagerAddr)
	if err != nil {
		return nil, fmt.Errorf("listen synthetic Alertmanager: %w", err)
	}
	fixture := &http.Server{Handler: demo.Alertmanager(time.Now), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	demoCtx, stop := context.WithCancel(ctx)
	go func() {
		if err := fixture.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("synthetic Alertmanager stopped: %v", err)
		}
	}()
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-demoCtx.Done():
				return
			case now := <-ticker.C:
				tickCtx, cancel := context.WithTimeout(demoCtx, 10*time.Second)
				err := demo.Tick(tickCtx, telemetry, now.UTC(), now.UTC())
				cancel()
				if err != nil && demoCtx.Err() == nil {
					log.Printf("synthetic telemetry update failed: %v", err)
				}
			}
		}
	}()
	return func() { stop(); _ = fixture.Close() }, nil
}
