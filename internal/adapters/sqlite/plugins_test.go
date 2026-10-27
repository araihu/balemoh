package sqlite

import (
	"context"
	"github.com/araihu/balemoh/migrations"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"path/filepath"
	"testing"

	"github.com/araihu/balemoh/internal/application/plugins"
)

func TestPluginConnectionReconciliationSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plugins.db")
	db, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err = RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	store := NewPluginStore(db)
	desired := []plugins.Provision{{PluginID: "example.plugin", Key: "lab", Name: "Lab", Digest: "hash-one", ManifestDigest: "manifest", Version: "0.0.1"}}
	rows, err := store.Reconcile(t.Context(), desired)
	if err != nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
	id := rows[0].ID
	if rows[0].Revision != "1" || !rows[0].Enabled || id == "" {
		t.Fatal("invalid initial identity")
	}
	db.Close()
	db, err = Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	store = NewPluginStore(db)
	rows, err = store.Reconcile(t.Context(), desired)
	if err != nil || rows[0].ID != id || rows[0].Revision != "1" {
		t.Fatal("replay changed identity/revision", err)
	}
	desired[0].Digest = "hash-two"
	rows, err = store.Reconcile(t.Context(), desired)
	if err != nil || rows[0].Revision != "2" || rows[0].ID != id {
		t.Fatal("config change did not advance", err)
	}
	for range 2 {
		rows, err = store.Reconcile(t.Context(), nil)
		if err != nil || rows[0].Enabled || rows[0].Revision != "3" || rows[0].ID != id {
			t.Fatal("omission deleted connection or repeated disable", err)
		}
	}
	rows, err = store.Reconcile(t.Context(), desired)
	if err != nil || !rows[0].Enabled || rows[0].Revision != "4" || rows[0].ID != id {
		t.Fatal("reenable replaced identity", err)
	}
	if _, err = store.Get(t.Context(), "other.plugin", id); plugins.ErrorCode(err) != "not_found" {
		t.Fatal("cross-plugin connection exposed")
	}
	if _, err = store.Reconcile(t.Context(), append(desired, desired[0])); plugins.ErrorCode(err) != "invalid_argument" {
		t.Fatal("duplicate provision accepted")
	}
	rows, _ = store.List(t.Context())
	if rows[0].Revision != "4" {
		t.Fatal("failed transaction modified connection")
	}
	// Exercise the restrictive relationship that persistent placements will own in P3.
	if _, err = db.ExecContext(t.Context(), `CREATE TABLE plugin_placement_fixture(id TEXT PRIMARY KEY,connection_id TEXT NOT NULL REFERENCES plugin_connections(id) ON DELETE RESTRICT)`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(t.Context(), `INSERT INTO plugin_placement_fixture VALUES('placement',?)`, id); err != nil {
		t.Fatal(err)
	}
	if err = store.Delete(context.Background(), id); plugins.ErrorCode(err) != "conflict" {
		t.Fatal("deleted referenced connection")
	}
}

func TestPluginMigrationDownUpRetainsConnections(t *testing.T) {
	db, err := Open(t.Context(), filepath.Join(t.TempDir(), "migration.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	store := NewPluginStore(db)
	rows, err := store.Reconcile(t.Context(), []plugins.Provision{{PluginID: "example.plugin", Key: "lab", Name: "Lab", Digest: "hash", ManifestDigest: "manifest", Version: "0.0.1"}})
	if err != nil {
		t.Fatal(err)
	}
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		t.Fatal(err)
	}
	migration, err := migrate.NewWithInstance("iofs", source, "sqlite3", driver)
	if err != nil {
		t.Fatal(err)
	}
	if err := migration.Steps(-1); err != nil {
		t.Fatal(err)
	}
	if err := migration.Up(); err != nil {
		t.Fatal("retained schema failed replay", err)
	}
	after, err := store.Get(t.Context(), "example.plugin", rows[0].ID)
	if err != nil || after.Revision != "1" {
		t.Fatal("down/up erased retained connection", err)
	}
}
