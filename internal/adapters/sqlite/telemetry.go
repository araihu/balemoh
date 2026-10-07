package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/araihu/balemoh/internal/application/telemetry"
)

type TelemetryStore struct{ db *sql.DB }

func NewTelemetryStore(db *sql.DB) *TelemetryStore { return &TelemetryStore{db: db} }
func (s *TelemetryStore) Register(ctx context.Context, sources []telemetry.Source) error {
	for _, source := range sources {
		h := telemetry.NewHost(source, "")
		body, _ := json.Marshal(h.Observation)
		if _, err := s.db.ExecContext(ctx, `INSERT INTO telemetry_hosts(id,source_kind,source_id,node,observation) VALUES(?,?,?,'',?) ON CONFLICT DO NOTHING`, h.ID, source.Kind, source.ID, string(body)); err != nil {
			return err
		}
	}
	return nil
}
func (s *TelemetryStore) Prune(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM telemetry_samples WHERE minute < ?`, now.Add(-24*time.Hour).Unix()/60)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM telemetry_hosts WHERE node <> '' AND received_at < ?`, now.Add(-24*time.Hour).UnixNano())
	return err
}
func (s *TelemetryStore) Ingest(ctx context.Context, b telemetry.Batch, now time.Time) error {
	if err := b.Validate(now); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var last int64
	err = tx.QueryRowContext(ctx, `SELECT observed_at FROM telemetry_hosts WHERE id=?`, telemetry.ID(b.Source, "")).Scan(&last)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if b.ObservedAt.UnixNano() <= last {
		return nil
	}
	// Remove expired nodes before enforcing the per-source cardinality bound.
	if _, err = tx.ExecContext(ctx, `DELETE FROM telemetry_hosts WHERE node <> '' AND received_at < ?`, now.Add(-24*time.Hour).UnixNano()); err != nil {
		return err
	}
	for _, observation := range b.Hosts {
		if observation.Workloads == nil {
			observation.Workloads = []telemetry.Workload{}
		}
		h := telemetry.NewHost(b.Source, observation.Node)
		body, _ := json.Marshal(observation)
		metrics, _ := json.Marshal(observation.Metrics)
		_, err = tx.ExecContext(ctx, `INSERT INTO telemetry_hosts(id,source_kind,source_id,node,observed_at,received_at,observation) VALUES(?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET observed_at=excluded.observed_at,received_at=excluded.received_at,observation=excluded.observation`, h.ID, b.Source.Kind, b.Source.ID, observation.Node, b.ObservedAt.UnixNano(), now.UnixNano(), string(body))
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO telemetry_samples(host_id,minute,metrics) VALUES(?,?,?) ON CONFLICT(host_id,minute) DO UPDATE SET metrics=excluded.metrics`, h.ID, b.ObservedAt.Unix()/60, string(metrics))
		if err != nil {
			return err
		}
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM telemetry_hosts WHERE source_kind=? AND source_id=?`, b.Source.Kind, b.Source.ID).Scan(&count); err != nil {
		return err
	}
	if count > 128 {
		return telemetry.ErrHostLimit
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM telemetry_samples WHERE minute < ?`, now.Add(-24*time.Hour).Unix()/60); err != nil {
		return err
	}
	return tx.Commit()
}

// ponytail: bounded homelab inventory is read together; project roots in SQL if inventory size makes this expensive.
func (s *TelemetryStore) List(ctx context.Context, now time.Time) ([]telemetry.Host, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT source_kind,source_id,node,observed_at,received_at,observation FROM telemetry_hosts ORDER BY source_kind,source_id,node`)
	if err != nil {
		return nil, err
	}
	result := []telemetry.Host{}
	seen := map[string]bool{}
	for rows.Next() {
		var source telemetry.Source
		var node, body string
		var observed, received int64
		if err = rows.Scan(&source.Kind, &source.ID, &node, &observed, &received, &body); err != nil {
			rows.Close()
			return nil, err
		}
		h := telemetry.NewHost(source, node)
		if err = json.Unmarshal([]byte(body), &h.Observation); err != nil {
			rows.Close()
			return nil, err
		}
		if observed != 0 {
			t := time.Unix(0, observed).UTC()
			h.ObservedAt = &t
		}
		if received != 0 {
			t := time.Unix(0, received).UTC()
			h.ReceivedAt = &t
		}
		if node != "" && received < now.Add(-24*time.Hour).UnixNano() {
			continue
		}
		h.SetStatus(now)
		result = append(result, h)
		seen[h.ID] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	// Legacy discovery sources stay visible before telemetry is enabled.
	rows, err = s.db.QueryContext(ctx, `SELECT DISTINCT source_kind,source_id FROM discovered_services`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var source telemetry.Source
		if err = rows.Scan(&source.Kind, &source.ID); err != nil {
			return nil, err
		}
		h := telemetry.NewHost(source, "")
		if !seen[h.ID] {
			result = append(result, h)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Kind != result[j].Kind {
			return result[i].Kind < result[j].Kind
		}
		return result[i].Name < result[j].Name
	})
	return result, rows.Err()
}
func (s *TelemetryStore) Detail(ctx context.Context, id, window string, now time.Time) (telemetry.Detail, error) {
	d := telemetry.Detail{Range: window, Children: []telemetry.Host{}, Samples: []telemetry.Sample{}}
	hosts, err := s.List(ctx, now)
	if err != nil {
		return d, err
	}
	for _, h := range hosts {
		if h.ID == id {
			d.Host = h
		}
		if h.ParentID == id {
			d.Children = append(d.Children, h)
		}
	}
	if d.Host.ID == "" {
		return d, sql.ErrNoRows
	}
	duration := time.Hour
	if window == "24h" {
		duration = 24 * time.Hour
	} else if window != "1h" {
		return d, fmt.Errorf("invalid range")
	}
	end := now.UTC().Truncate(time.Minute)
	start := end.Add(-duration).Add(time.Minute)
	rows, err := s.db.QueryContext(ctx, `SELECT minute,metrics FROM telemetry_samples WHERE host_id=? AND minute>=? AND minute<=? ORDER BY minute`, id, start.Unix()/60, end.Unix()/60)
	if err != nil {
		return d, err
	}
	defer rows.Close()
	samples := map[int64]telemetry.Metrics{}
	for rows.Next() {
		var minute int64
		var body string
		var m telemetry.Metrics
		if err = rows.Scan(&minute, &body); err != nil {
			return d, err
		}
		if err = json.Unmarshal([]byte(body), &m); err != nil {
			return d, err
		}
		samples[minute] = m
	}
	if err = rows.Err(); err != nil {
		return d, err
	}
	for at := start; !at.After(end); at = at.Add(time.Minute) {
		d.Samples = append(d.Samples, telemetry.Sample{At: at, Metrics: samples[at.Unix()/60]})
	}
	return d, nil
}
