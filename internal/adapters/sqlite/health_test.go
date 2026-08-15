package sqlite

import (
	"context"
	"testing"

	"github.com/araihu/balemoh/internal/storage/sqlc"
)

func TestPingerPing(t *testing.T) {
	db, err := Open(context.Background(), t.TempDir()+"/balemoh.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	pinger := NewPinger(sqlc.New(db))
	if err := pinger.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
}
