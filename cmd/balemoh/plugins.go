package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	httpadapter "github.com/araihu/balemoh/internal/adapters/http"
	pluginadapter "github.com/araihu/balemoh/internal/adapters/plugins"
	"github.com/araihu/balemoh/internal/adapters/sqlite"
	"github.com/araihu/balemoh/internal/application/catalog"
	"github.com/araihu/balemoh/internal/application/dashboard"
	"github.com/araihu/balemoh/internal/application/notifications"
	"github.com/araihu/balemoh/internal/application/plugins"
	"github.com/araihu/balemoh/internal/config"
)

func pluginDeployment(options config.Options) (*pluginadapter.Deployment, string, error) {
	if options.PluginConfig == "" {
		return nil, "", nil
	}
	token, err := pluginadapter.ReadCredential(options.PluginServiceTokenFile)
	if err != nil || len(token) < 32 {
		return nil, "", errors.New("invalid plugin service credential file")
	}
	deployment, err := pluginadapter.Load(options.PluginConfig)
	if err == nil && options.DemoEnabled {
		err = deployment.ValidateDemo("http://" + options.DemoAlertmanagerAddr)
	}
	return deployment, token, err
}
func startPlugins(ctx context.Context, db *sql.DB, server *http.Server, deployment *pluginadapter.Deployment, token string, catalogService *catalog.Service, notificationService *notifications.Service, demoEnabled bool) (func(), error) {
	store := sqlite.NewPluginStore(db)
	records, err := store.Reconcile(ctx, deployment.Provisions())
	if err != nil {
		return nil, errors.New("cannot reconcile plugin connections")
	}
	runtime := pluginadapter.NewRuntime(ctx, deployment, records, notificationService)
	if deployment != nil {
		service := plugins.New(store, runtime)
		if demoEnabled {
			for _, record := range records {
				if record.PluginID != "araihu.alertmanager" || record.Key != "demo" {
					continue
				}
				for index, contribution := range []string{"summary", "active-list"} {
					id := fmt.Sprintf("00000000-0000-4000-8000-%012d", index+1)
					if _, _, err := service.PutPlacement(ctx, id, plugins.PlacementBody{PluginID: record.PluginID, ConnectionID: record.ID, ContributionID: contribution, Options: []byte("{}"), Enabled: true}, ""); err != nil {
						runtime.Close()
						return nil, fmt.Errorf("seed demo plugin widgets: %w", err)
					}
				}
			}
		}
		server.Handler = httpadapter.MountPlugins(server.Handler, service, token, dashboard.ServiceProvider{Catalog: catalogService}, dashboard.HostProvider{Hosts: sqlite.NewTelemetryStore(db), Selections: sqlite.NewDashboardSelections(db)})
	}
	return runtime.Close, nil
}
