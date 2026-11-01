package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/araihu/balemoh/internal/application/catalog"
	"github.com/araihu/balemoh/internal/application/dashboard"
	"github.com/araihu/balemoh/internal/storage/sqlc"
	"math"
	"strconv"
	"time"
)

type LayoutStore struct {
	db          *sql.DB
	PluginTitle func(string, string) string
}

func NewLayoutStore(db *sql.DB) *LayoutStore { return &LayoutStore{db: db} }
func layoutUnavailable() error {
	return dashboard.LayoutFailure(503, "unavailable", "Layout storage or selected identities are unavailable.")
}
func readLayout(ctx context.Context, tx *sql.Tx) (dashboard.LayoutState, error) {
	result := dashboard.LayoutState{Revision: "0", Document: dashboard.DefaultLayout()}
	var version, revision int64
	var raw string
	e := tx.QueryRowContext(ctx, `SELECT schema_version,revision,document FROM dashboard_layout WHERE id=1`).Scan(&version, &revision, &raw)
	if errors.Is(e, sql.ErrNoRows) {
		return result, nil
	}
	if e != nil {
		return result, layoutUnavailable()
	}
	unreadable := func() (dashboard.LayoutState, error) {
		return dashboard.LayoutState{}, dashboard.LayoutFailure(503, "layout_unreadable", "Saved layout is unreadable. Use the temporary default view or restore a compatible backup.")
	}
	if version != 1 || revision < 1 || dashboard.DecodeLayoutJSON([]byte(raw), &result.Document) != nil {
		return unreadable()
	}
	previous := dashboard.CloneLayout(result.Document)
	if dashboard.ValidateLayout(&result.Document, previous, nil) != nil {
		return unreadable()
	}
	result.Revision = strconv.FormatInt(revision, 10)
	return result, nil
}
func (s *LayoutStore) identities(ctx context.Context, tx *sql.Tx) ([]dashboard.Widget, error) {
	candidates, e := (&CatalogStore{db: s.db}).listCandidates(ctx, sqlc.New(tx), false)
	if e != nil {
		return nil, e
	}
	items := []dashboard.Widget{}
	for _, c := range catalog.HomepageCandidates(candidates) {
		items = append(items, dashboard.Widget{ID: "service-" + c.ID, SourceKind: "service", SourceID: c.ID, Kind: "service-links", Presentation: "card", Title: c.DisplayName})
	}
	rows, e := tx.QueryContext(ctx, `SELECT d.source_id,COALESCE(NULLIF(h.node,''),h.source_id,d.source_id) FROM dashboard_selections d LEFT JOIN telemetry_hosts h ON h.id=d.source_id WHERE d.source_kind='host' ORDER BY d.source_id`)
	if e != nil {
		return nil, e
	}
	for rows.Next() {
		var id, title string
		if e = rows.Scan(&id, &title); e != nil {
			rows.Close()
			return nil, e
		}
		items = append(items, dashboard.Widget{ID: "host-" + id, SourceKind: "host", SourceID: id, Kind: "host-metrics", Presentation: "card", Title: title})
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	placements, e := sqlc.New(tx).ListPluginPlacements(ctx)
	if e != nil {
		return nil, e
	}
	for _, p := range placements {
		if p.Enabled == 0 {
			continue
		}
		id := "plugin-" + p.ID
		title := id
		if s.PluginTitle != nil {
			if v := s.PluginTitle(p.PluginID, p.ContributionID); v != "" {
				title = v
			}
		}
		items = append(items, dashboard.Widget{ID: id, SourceKind: "plugin", SourceID: p.ID, Kind: "plugin-widget", Presentation: "plugin-ui", Title: title})
	}
	return items, nil
}
func (s *LayoutStore) Read(ctx context.Context, project bool) (dashboard.LayoutState, []dashboard.Widget, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return dashboard.LayoutState{}, nil, layoutUnavailable()
	}
	defer tx.Rollback()
	state, e := readLayout(ctx, tx)
	if e != nil {
		return state, nil, e
	}
	var items []dashboard.Widget
	if project {
		items, e = s.identities(ctx, tx)
	}
	if e != nil {
		return state, nil, layoutUnavailable()
	}
	if e = tx.Commit(); e != nil {
		return state, nil, layoutUnavailable()
	}
	return state, items, nil
}
func (s *LayoutStore) ReadDefault(ctx context.Context) ([]dashboard.Widget, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, layoutUnavailable()
	}
	defer tx.Rollback()
	items, e := s.identities(ctx, tx)
	if e != nil {
		return nil, layoutUnavailable()
	}
	if e = tx.Commit(); e != nil {
		return nil, layoutUnavailable()
	}
	return items, nil
}
func (s *LayoutStore) reserve(ctx context.Context) (*sql.Tx, error) {
	for {
		tx, e := s.db.BeginTx(ctx, nil)
		if e != nil {
			return nil, e
		}
		_, e = tx.ExecContext(ctx, `UPDATE dashboard_layout SET revision=revision WHERE 0`)
		if e == nil {
			return tx, nil
		}
		_ = tx.Rollback()
		var code interface{ Code() int }
		if !errors.As(e, &code) || code.Code()&255 != 5 {
			return nil, e
		}
		timer := time.NewTimer(15 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}
func (s *LayoutStore) Update(ctx context.Context, expected string, fn func(dashboard.LayoutState, []dashboard.Widget) (dashboard.LayoutDocument, error)) (dashboard.LayoutState, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, e := s.reserve(ctx)
	if e != nil {
		return dashboard.LayoutState{}, layoutUnavailable()
	}
	defer tx.Rollback()
	current, e := readLayout(ctx, tx)
	if e != nil {
		return current, e
	}
	if current.Revision != expected {
		return current, dashboard.LayoutFailure(412, "revision_conflict", "Layout changed. Preserve the draft and inspect the saved version.")
	}
	items, e := s.identities(ctx, tx)
	if e != nil {
		return current, layoutUnavailable()
	}
	document, e := fn(current, items)
	if e != nil {
		return current, e
	}
	raw, e := json.Marshal(document)
	if e != nil {
		return current, layoutUnavailable()
	}
	old, _ := json.Marshal(current.Document)
	if bytes.Equal(raw, old) {
		return current, nil
	}
	revision, _ := strconv.ParseInt(current.Revision, 10, 64)
	if revision == math.MaxInt64 {
		return current, dashboard.LayoutFailure(409, "limit_exceeded", "Layout revision limit reached.")
	}
	revision++
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var result sql.Result
	if current.Revision == "0" {
		result, e = tx.ExecContext(ctx, `INSERT INTO dashboard_layout(id,schema_version,revision,document,updated_at) VALUES(1,1,?,?,?)`, revision, string(raw), now)
	} else {
		result, e = tx.ExecContext(ctx, `UPDATE dashboard_layout SET revision=?,document=?,updated_at=? WHERE id=1 AND revision=?`, revision, string(raw), now, current.Revision)
	}
	if e != nil {
		return current, layoutUnavailable()
	}
	n, e := result.RowsAffected()
	if e != nil || n != 1 {
		return current, layoutUnavailable()
	}
	if e = tx.Commit(); e != nil {
		return current, layoutUnavailable()
	}
	return dashboard.LayoutState{Revision: strconv.FormatInt(revision, 10), Document: document}, nil
}
