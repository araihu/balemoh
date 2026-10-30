package demo_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/araihu/balemoh/client/iconassets"
	"github.com/araihu/balemoh/internal/adapters/sqlite"
	"github.com/araihu/balemoh/internal/application/catalog"
	"github.com/araihu/balemoh/internal/demo"
)

func TestScenarioRebuildReplayHistoryAndRetention(t *testing.T) {
	ctx := t.Context()
	db, err := sqlite.Open(ctx, t.TempDir()+"/demo.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := sqlite.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	c := catalog.NewService(sqlite.NewCatalogStore(db))
	telemetry := sqlite.NewTelemetryStore(db)
	selections := sqlite.NewDashboardSelections(db)
	now := time.Now().UTC().Truncate(time.Minute)
	for range 2 {
		if err := demo.Seed(ctx, c, telemetry, selections, now); err != nil {
			t.Fatal(err)
		}
		services, err := c.ListHomepage(ctx)
		if err != nil || len(services) != 12 {
			t.Fatalf("homepage=%d err=%v", len(services), err)
		}
		for _, service := range services {
			icon, ok := iconassets.Lookup(service.Icon)
			if !ok || !strings.HasPrefix(service.Icon, "selfhst:") {
				t.Fatalf("demo icon is not a bundled service brand: %s", service.Icon)
			}
			for _, variant := range icon.Variants {
				if variant.Appearance != "default" {
					continue
				}
				data, err := fs.ReadFile(iconassets.DemoFiles(), strings.TrimPrefix(variant.Path, "/ui/icon-library/selfhst/"))
				if err != nil || fmt.Sprintf("%x", sha256.Sum256(data)) != variant.SHA256 {
					t.Fatalf("demo image unavailable or differs from locked catalog: %s: %v", service.Icon, err)
				}
			}
		}
		roots, err := telemetry.List(ctx, now)
		if err != nil || len(roots) != 5 {
			t.Fatalf("roots=%d err=%v", len(roots), err)
		}
		selected, err := selections.List(ctx, "host")
		if err != nil || len(selected) != 2 {
			t.Fatalf("selections=%v err=%v", selected, err)
		}
		for _, host := range roots {
			detail, err := telemetry.Detail(ctx, host.ID, "1h", now)
			if err != nil {
				t.Fatal(err)
			}
			measured := 0
			for _, sample := range detail.Samples {
				if sample.CPUPercent != nil {
					measured++
				}
			}
			if measured != 60 {
				t.Fatalf("expected populated hour, got %d", measured)
			}
			if host.Kind == "cluster" && len(detail.Children) != 3 {
				t.Fatalf("children=%d", len(detail.Children))
			}
		}
	}
	future := now.Add(25 * time.Hour)
	if err := demo.Tick(ctx, telemetry, future, future); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM telemetry_samples").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 5 {
		t.Fatalf("expired history retained: %d rows", count)
	}
}

func TestAlertmanagerFixtureIsBoundedAndReadOnly(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	h := demo.Alertmanager(func() time.Time { return now })
	response := httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequest("GET", "/api/v2/alerts", nil))
	var alerts []struct {
		Fingerprint string
		Annotations map[string]string
		Status      struct{ State string }
	}
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &alerts) != nil || len(alerts) != 3 {
		t.Fatalf("fixture=%d %s", response.Code, response.Body.String())
	}
	if len(alerts[0].Annotations["balemoh_service_id"]) != 64 || alerts[2].Status.State != "suppressed" {
		t.Fatal("missing catalog reference or suppression")
	}
	response = httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequest("POST", "/api/v2/alerts", nil))
	if response.Code != 405 {
		t.Fatalf("fixture accepted mutation: %d", response.Code)
	}
}
