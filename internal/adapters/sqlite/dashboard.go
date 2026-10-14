package sqlite

import (
	"context"
	"database/sql"

	"github.com/araihu/balemoh/internal/application/dashboard"
)

type DashboardSelections struct{ db *sql.DB }

func NewDashboardSelections(db *sql.DB) *DashboardSelections { return &DashboardSelections{db: db} }
func (s *DashboardSelections) List(ctx context.Context, kind string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT source_id FROM dashboard_selections WHERE source_kind=? ORDER BY source_id`, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
func (s *DashboardSelections) Set(ctx context.Context, kind, id string, enabled bool) error {
	if !enabled {
		_, err := s.db.ExecContext(ctx, `DELETE FROM dashboard_selections WHERE source_kind=? AND source_id=?`, kind, id)
		return err
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO dashboard_selections(source_kind,source_id) SELECT ?,? WHERE (SELECT COUNT(*) FROM dashboard_selections)<128 ON CONFLICT DO NOTHING`, kind, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil || n > 0 {
		return err
	}
	var exists bool
	err = s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM dashboard_selections WHERE source_kind=? AND source_id=?)`, kind, id).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return dashboard.ErrLimit
	}
	return nil
}
