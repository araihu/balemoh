package sqlite

import (
	"context"
	"testing"
	"testing/fstest"
)

func TestRunMigrations(t *testing.T) {
	db, err := Open(context.Background(), t.TempDir()+"/balemoh.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations() first call error = %v", err)
	}
	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations() second call error = %v", err)
	}

	var version uint
	var dirty bool
	if err := db.QueryRowContext(context.Background(), "SELECT version, dirty FROM schema_migrations").Scan(&version, &dirty); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if version != 1 || dirty {
		t.Fatalf("schema_migrations = (version %d, dirty %t), want (1, false)", version, dirty)
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
