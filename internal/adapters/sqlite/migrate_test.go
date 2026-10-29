package sqlite

import (
	"context"
	"github.com/araihu/balemoh/migrations"
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
	if version != 17 || dirty {
		t.Fatalf("schema_migrations = (version %d, dirty %t), want (17, false)", version, dirty)
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

func TestSeparateLogoMigrationPreservesLegacyBranding(t *testing.T) {
	db, err := Open(t.Context(), t.TempDir()+"/legacy.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	files := fstest.MapFS{}
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() >= "000015" {
			continue
		}
		data, err := migrations.FS.ReadFile(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		files[entry.Name()] = &fstest.MapFile{Data: data}
	}
	if err = runMigrations(db, files); err != nil {
		t.Fatal(err)
	}
	image, err := NewIconStore(db).Save(t.Context(), "", "Legacy", "", "", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><circle r="8"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO presentation_themes(id,name,schema_version,definition_json,provenance_json,revision,creation_digest,brand_icon_id) VALUES('legacy','Legacy',1,'{}','{}',1,'legacy',?)`, image.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	var logo, icon string
	if err = db.QueryRow("SELECT brand_logo_id,brand_icon_id FROM presentation_themes WHERE id='legacy'").Scan(&logo, &icon); err != nil || logo != image.ID || icon != image.ID {
		t.Fatal("migration changed branding", logo, icon, err)
	}
}

func TestNotificationReferenceMigrationPreservesSchema15Inbox(t *testing.T) {
	db, err := Open(t.Context(), t.TempDir()+"/previous.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	old := fstest.MapFS{}
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() >= "000016" {
			continue
		}
		data, e := migrations.FS.ReadFile(entry.Name())
		if e != nil {
			t.Fatal(e)
		}
		old[entry.Name()] = &fstest.MapFile{Data: data}
	}
	if err = runMigrations(db, old); err != nil {
		t.Fatal(err)
	}
	payload := `{"original":"unchanged"}`
	_, err = db.ExecContext(t.Context(), `INSERT INTO notifications(id,source_kind,source_id,connection_id,event_id,severity,title,body,occurred_at,received_at,destination_json,payload_json,read_at) VALUES ('original-id','external','original-source','','event','warning','Keep title','Keep body','2026-10-29T00:00:00Z','2026-10-29T00:00:00Z','{}',?,'2026-10-29T00:01:00Z')`, payload)
	if err != nil {
		t.Fatal(err)
	}
	if err = RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	var reference, body, title, stored, readAt string
	err = db.QueryRowContext(t.Context(), `SELECT service_id,body,title,payload_json,read_at FROM notifications WHERE id='original-id'`).Scan(&reference, &body, &title, &stored, &readAt)
	if err != nil || reference != "" || body != "Keep body" || title != "Keep title" || stored != payload || readAt != "2026-10-29T00:01:00Z" {
		t.Fatal(reference, body, title, stored, readAt, err)
	}
}
