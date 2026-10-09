package sqlite

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/araihu/balemoh/internal/application/telemetry"
)

func TestTelemetryHistoryReplayRetentionAndIsolation(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, t.TempDir()+"/hosts.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	store := NewTelemetryStore(db)
	source := telemetry.Source{Kind: "kubernetes", ID: "cluster"}
	now := time.Now().UTC().Truncate(time.Minute).Add(10 * time.Second)
	if err = store.Register(ctx, []telemetry.Source{source, {Kind: "container", ID: "empty"}}); err != nil {
		t.Fatal(err)
	}
	hosts, err := store.List(ctx, now)
	if err != nil || len(hosts) != 2 {
		t.Fatalf("registered inventory: %v %v", hosts, err)
	}
	zero := 0.0
	memory, total := int64(0), int64(1024)
	batch := telemetry.Batch{Source: source, ObservedAt: now.Add(-2 * time.Minute), Hosts: []telemetry.Observation{{Metrics: telemetry.Metrics{CPUPercent: &zero, MemoryUsedBytes: &memory, MemoryTotalBytes: &total}}, {Node: "node-a"}}}
	if err = store.Ingest(ctx, batch, now); err != nil {
		t.Fatal(err)
	}
	id := telemetry.ID(source, "")
	d, err := store.Detail(ctx, id, "1h", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Samples) != 60 || len(d.Children) != 1 || d.Samples[57].CPUPercent == nil || *d.Samples[57].CPUPercent != 0 || d.Samples[58].CPUPercent != nil {
		t.Fatal("zero must survive; missing minute must remain null")
	}
	received := *d.Host.ReceivedAt
	if err = store.Ingest(ctx, batch, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	d, _ = store.Detail(ctx, id, "1h", now.Add(time.Minute))
	if !d.Host.ReceivedAt.Equal(received) {
		t.Fatal("replay refreshed heartbeat")
	}
	// A later observation in the same minute replaces that bucket; it does not grow history.
	value := 20.0
	batch.ObservedAt = batch.ObservedAt.Add(time.Second)
	batch.Hosts[0].CPUPercent = &value
	if err = store.Ingest(ctx, batch, now); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM telemetry_samples WHERE host_id=?`, id).Scan(&count); err != nil || count != 1 {
		t.Fatalf("minute replacement: %d %v", count, err)
	}
	// Invalid batch is atomic, including its otherwise valid root.
	batch.ObservedAt = now
	batch.Hosts[1].CPUPercent = new(float64(-1))
	if store.Ingest(ctx, batch, now) == nil {
		t.Fatal("negative metric accepted")
	}
	d, _ = store.Detail(ctx, id, "1h", now)
	if !d.Host.ObservedAt.Equal(now.Add(-2*time.Minute + time.Second)) {
		t.Fatal("invalid batch modified root")
	}
	if err = store.Prune(ctx, now.Add(25*time.Hour)); err != nil {
		t.Fatal(err)
	}
	d, err = store.Detail(ctx, id, "24h", now.Add(25*time.Hour))
	if err != nil || len(d.Samples) != 1440 || len(d.Children) != 0 || d.Host.Status != "stale" {
		t.Fatalf("retention: %+v %v", d.Host, err)
	}
	if err = db.QueryRow(`SELECT count(*) FROM telemetry_samples`).Scan(&count); err != nil || count != 0 {
		t.Fatal("expired samples remain")
	}
	if err = db.QueryRow(`SELECT count(*) FROM discovered_services`).Scan(&count); err != nil || count != 0 {
		t.Fatal("telemetry changed catalog")
	}
}
func TestTelemetryRejectsSourceCardinalityGrowth(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, t.TempDir()+"/hosts.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	store := NewTelemetryStore(db)
	now := time.Now().UTC()
	b := telemetry.Batch{Source: telemetry.Source{Kind: "kubernetes", ID: "cluster"}, ObservedAt: now, Hosts: []telemetry.Observation{{}}}
	for i := 0; i < 127; i++ {
		b.Hosts = append(b.Hosts, telemetry.Observation{Node: fmt.Sprintf("node-%d", i)})
	}
	if err = store.Ingest(ctx, b, now); err != nil {
		t.Fatal(err)
	}
	b.ObservedAt = now.Add(time.Second)
	b.Hosts = []telemetry.Observation{{}, {Node: "new-node"}}
	if store.Ingest(ctx, b, now.Add(time.Second)) == nil {
		t.Fatal("unbounded node growth accepted")
	}
	hosts, err := store.List(ctx, now)
	if err != nil || len(hosts) != 128 {
		t.Fatal("failed ingestion was not atomic")
	}
}
