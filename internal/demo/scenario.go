// Package demo supplies a bounded fictional homelab through the normal domain
// contracts. It never discovers or contacts external services.
package demo

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/araihu/balemoh/internal/application/catalog"
	"github.com/araihu/balemoh/internal/application/telemetry"
)

var sources = []telemetry.Source{{Kind: "kubernetes", ID: "demo-cluster"}, {Kind: "container", ID: "demo-docker"}}
var names = []string{"grafana", "prometheus", "jellyfin", "immich", "forgejo", "paperless", "vaultwarden", "home-assistant", "uptime-kuma", "gitea-runner", "minio", "postgres"}

type Catalog interface {
	ImportSnapshot(context.Context, catalog.Snapshot) (catalog.SyncResult, error)
	Pin(context.Context, string) (catalog.Candidate, error)
	Edit(context.Context, string, catalog.Edit) error
}
type Telemetry interface {
	Register(context.Context, []telemetry.Source) error
	Ingest(context.Context, telemetry.Batch, time.Time) error
	Prune(context.Context, time.Time) error
}
type Selections interface {
	Set(context.Context, string, string, bool) error
}

func candidate(index int, now time.Time) catalog.Candidate {
	source := sources[index/6]
	kind, namespace := "Service", "demo"
	if source.Kind == "container" {
		kind, namespace = "container", ""
	}
	c := catalog.NewCandidate(catalog.SourceRef{Kind: source.Kind, ID: source.ID}, catalog.ResourceRef{Kind: kind, Namespace: namespace, Name: names[index]}, now)
	c.DisplayName = names[index]
	c.Description = "Synthetic demo service. No production data or live destination."
	c.Images = []string{"registry.example.test/demo/" + names[index] + ":1.0.0"}
	c.Endpoints = []catalog.Endpoint{{Name: "web", URL: "https://" + names[index] + ".example.test", Protocol: "https", Port: 443, Provenance: "synthetic-demo"}}
	return c
}

// Seed uses idempotent source import and selection operations. Backfill is
// bounded to 60 one-minute samples and accepted timestamps within the last day.
func Seed(ctx context.Context, c Catalog, t Telemetry, selections Selections, now time.Time) error {
	if err := t.Register(ctx, sources); err != nil {
		return err
	}
	for group, source := range sources {
		snapshot := catalog.Snapshot{Source: catalog.SourceRef{Kind: source.Kind, ID: source.ID}, ObservedAt: now, Candidates: []catalog.Candidate{}}
		for i := group * 6; i < (group+1)*6; i++ {
			snapshot.Candidates = append(snapshot.Candidates, candidate(i, now))
		}
		if _, err := c.ImportSnapshot(ctx, snapshot); err != nil {
			return err
		}
		for i := group * 6; i < (group+1)*6; i++ {
			if _, err := c.Pin(ctx, candidate(i, now).ID); err != nil {
				return err
			}
			item := candidate(i, now)
			icons := []string{"grafana", "prometheus", "jellyfin", "immich", "forgejo", "paperless-ngx", "vaultwarden", "home-assistant", "uptime-kuma", "gitea", "minio", "postgresql"}
			icon := "selfhst:" + icons[i]
			if err := c.Edit(ctx, item.ID, catalog.Edit{DisplayName: item.DisplayName, Description: item.Description, Icon: &icon}); err != nil {
				return err
			}
		}
		if err := selections.Set(ctx, "host", telemetry.ID(source, ""), true); err != nil {
			return err
		}
	}
	for minute := 59; minute >= 0; minute-- {
		if err := Tick(ctx, t, now.Add(-time.Duration(minute)*time.Minute), now); err != nil {
			return err
		}
	}
	return nil
}

func ptr[T any](value T) *T { return &value }
func metrics(now time.Time, offset int, total int64) telemetry.Metrics {
	phase := float64(now.Unix()/60%240)/12 + float64(offset)
	cpu := 35 + 22*math.Sin(phase)
	used := int64(float64(total) * (0.48 + 0.1*math.Sin(phase/3)))
	return telemetry.Metrics{CPUPercent: ptr(cpu), MemoryUsedBytes: ptr(used), MemoryTotalBytes: ptr(total)}
}
func Batches(now time.Time) []telemetry.Batch {
	result := []telemetry.Batch{}
	for group, source := range sources {
		hosts := []telemetry.Observation{}
		count := 1
		if source.Kind == "kubernetes" {
			count = 4
		}
		for node := 0; node < count; node++ {
			name, arch, total := "", "amd64", int64(8<<30)
			if source.Kind == "kubernetes" {
				total = 24 << 30
				arch = ""
			}
			if node > 0 {
				name = fmt.Sprintf("demo-node-%02d", node)
				total = 8 << 30
				arch = "amd64"
				if node == 3 {
					arch = "arm64"
				}
			}
			h := telemetry.Observation{Node: name, Architecture: arch, Ready: "True", InventoryAvailable: true, Metrics: metrics(now, group+node, total), Workloads: []telemetry.Workload{}}
			if source.Kind == "kubernetes" && node == 1 {
				h.MemoryUsedBytes = ptr(int64(float64(total) * (0.84 + 0.03*math.Sin(float64(now.Unix()/60%240)/12))))
			}
			for i := group * 6; i < (group+1)*6; i++ {
				nodeName := ""
				if source.Kind == "kubernetes" {
					nodeName = fmt.Sprintf("demo-node-%02d", i%3+1)
					if node > 0 && nodeName != name {
						continue
					}
				}
				state, reason := "Running", ""
				restarts := 0
				if i == 1 {
					restarts = 2
					reason = "Synthetic restart example"
				}
				h.Workloads = append(h.Workloads, telemetry.Workload{Name: names[i], Namespace: map[bool]string{true: "demo"}[source.Kind == "kubernetes"], State: state, Restarts: restarts, Ready: ptr(true), Reason: reason, Node: nodeName, Images: candidate(i, now).Images})
			}
			hosts = append(hosts, h)
		}
		// Cluster measurements are aggregates of the same three node observations.
		if count == 4 {
			cpu := 0.0
			var used int64
			for _, h := range hosts[1:] {
				cpu += *h.CPUPercent / 3
				used += *h.MemoryUsedBytes
			}
			hosts[0].CPUPercent = ptr(cpu)
			hosts[0].MemoryUsedBytes = ptr(used)
		}
		result = append(result, telemetry.Batch{Source: source, ObservedAt: now, Hosts: hosts})
	}
	return result
}
func Tick(ctx context.Context, t Telemetry, observed, received time.Time) error {
	for _, batch := range Batches(observed) {
		if err := t.Ingest(ctx, batch, received); err != nil {
			return err
		}
	}
	return t.Prune(ctx, received)
}

// Alertmanager serves only the read operations consumed by the real plugin.
// Responses use synthetic IDs and stable startsAt values for each UTC day.
func Alertmanager(now func() time.Time) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v2/alerts", func(w http.ResponseWriter, r *http.Request) {
		at := now().UTC()
		start := at.Truncate(24 * time.Hour).Add(-time.Hour)
		alerts := []map[string]any{}
		for i, severity := range []string{"warning", "critical", "warning"} {
			inhibited := []string{}
			state := "active"
			if i == 2 {
				inhibited = []string{"0000000000000001"}
				state = "suppressed"
			}
			alerts = append(alerts, map[string]any{
				"fingerprint": fmt.Sprintf("%016x", i+1), "startsAt": start.Format(time.RFC3339), "endsAt": at.Add(time.Hour).Format(time.RFC3339), "updatedAt": at.Format(time.RFC3339),
				"labels":      map[string]string{"alertname": []string{"DemoHighMemory", "DemoServiceRestarts", "DemoSuppressedAlert"}[i], "severity": severity, "namespace": "demo", "instance": fmt.Sprintf("demo-node-%02d", i%3+1)},
				"annotations": map[string]string{"summary": []string{"Memory pressure on demo node", "Service restarted twice", "Suppressed synthetic warning"}[i], "description": "Synthetic alert for the public Balemoh demo.", "balemoh_service_id": candidate(i, at).ID},
				"status":      map[string]any{"state": state, "silencedBy": []string{}, "inhibitedBy": inhibited}, "receivers": []map[string]string{{"name": "demo"}}, "generatorURL": "https://alerts.example.test"})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(alerts)
	})
	return mux
}
