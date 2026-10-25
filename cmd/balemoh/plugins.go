package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	httpadapter "github.com/araihu/balemoh/internal/adapters/http"
	pluginadapter "github.com/araihu/balemoh/internal/adapters/plugins"
	"github.com/araihu/balemoh/internal/adapters/sqlite"
	"github.com/araihu/balemoh/internal/application/catalog"
	"github.com/araihu/balemoh/internal/application/dashboard"
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
	return deployment, token, err
}
func startPlugins(ctx context.Context, db *sql.DB, server *http.Server, deployment *pluginadapter.Deployment, token string, catalogService *catalog.Service) (func(), error) {
	store := sqlite.NewPluginStore(db)
	records, err := store.Reconcile(ctx, deployment.Provisions())
	if err != nil {
		return nil, errors.New("cannot reconcile plugin connections")
	}
	runtime := pluginadapter.NewRuntime(ctx, deployment, records)
	if deployment != nil {
		server.Handler = httpadapter.MountPlugins(server.Handler, plugins.New(store, runtime), token, dashboard.ServiceProvider{Catalog: catalogService}, dashboard.HostProvider{Hosts: sqlite.NewTelemetryStore(db), Selections: sqlite.NewDashboardSelections(db)})
	}
	return runtime.Close, nil
}
