package sqlite

import (
	"context"
	"testing"
	"testing/fstest"
)

func TestRunMigrations(t *testing.T) {
	databasePath := t.TempDir() + "/balemoh.db"
	db, err := Open(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations() first call error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	db, err = Open(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("Open() second call error = %v", err)
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations() restart call error = %v", err)
	}

	var version uint
	var dirty bool
	if err := db.QueryRowContext(context.Background(), "SELECT version, dirty FROM schema_migrations").Scan(&version, &dirty); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if version != 7 || dirty {
		t.Fatalf("schema_migrations = (version %d, dirty %t), want (7, false)", version, dirty)
	}
}

func TestCatalogSchemaEnforcesServiceForeignKey(t *testing.T) {
	db, err := Open(context.Background(), t.TempDir()+"/balemoh.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	for _, table := range []string{"discovered_services", "service_endpoints", "discovery_source_snapshots"} {
		var name string
		if err := db.QueryRowContext(context.Background(), "SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&name); err != nil {
			t.Fatalf("query table %q: %v", table, err)
		}
		if name != table {
			t.Fatalf("table name = %q, want %q", name, table)
		}
	}

	var columnName string
	if err := db.QueryRowContext(context.Background(), "SELECT name FROM pragma_table_info('discovered_services') WHERE name = 'images_json'").Scan(&columnName); err != nil {
		t.Fatalf("query images_json column: %v", err)
	}
	if columnName != "images_json" {
		t.Fatalf("column name = %q, want images_json", columnName)
	}

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO service_endpoints (service_id, name, url, port, protocol, provenance)
		VALUES ('missing', 'web', '', 0, '', '')
	`); err == nil {
		t.Fatal("orphan endpoint insert succeeded, want foreign-key error")
	}
}

func TestRunMigrationsInvalidSource(t *testing.T) {
	db, err := Open(context.Background(), t.TempDir()+"/balemoh.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	files := fstest.MapFS{
		"000001_bad.up.sql": &fstest.MapFile{Data: []byte("NOT VALID SQL;")},
	}
	if err := runMigrations(db, files); err == nil {
		t.Fatal("runMigrations() error = nil, want error")
	}
}
