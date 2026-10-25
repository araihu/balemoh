package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"github.com/araihu/balemoh/internal/application/plugins"
	"github.com/araihu/balemoh/internal/storage/sqlc"
	"strconv"
	"time"
)

const placementColumns = `id,plugin_id,connection_id,contribution_id,options,enabled,revision,created_at,updated_at`

func scanPlacement(row pluginScanner) (plugins.Placement, error) {
	var p plugins.Placement
	var revision int64
	var created, updated, options string
	err := row.Scan(&p.PlacementID, &p.PluginID, &p.ConnectionID, &p.ContributionID, &options, &p.Enabled, &revision, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return p, plugins.Fail("not_found")
	}
	if err != nil {
		return p, err
	}
	p.Options = []byte(options)
	p.Revision = strconv.FormatInt(revision, 10)
	p.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	p.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return p, nil
}
func (s *PluginStore) Placements(ctx context.Context) ([]plugins.Placement, error) {
	rows, err := sqlc.New(s.db).ListPluginPlacements(ctx)
	if err != nil {
		return nil, err
	}
	out := []plugins.Placement{}
	for _, v := range rows {
		created, _ := time.Parse(time.RFC3339Nano, v.CreatedAt)
		updated, _ := time.Parse(time.RFC3339Nano, v.UpdatedAt)
		out = append(out, plugins.Placement{PlacementID: v.ID, PlacementBody: plugins.PlacementBody{PluginID: v.PluginID, ConnectionID: v.ConnectionID, ContributionID: v.ContributionID, Options: []byte(v.Options), Enabled: v.Enabled != 0}, Revision: strconv.FormatInt(v.Revision, 10), CreatedAt: created, UpdatedAt: updated})
	}
	return out, nil
}

func (s *PluginStore) Placement(ctx context.Context, id string) (plugins.Placement, error) {
	return scanPlacement(s.db.QueryRowContext(ctx, `SELECT `+placementColumns+` FROM plugin_placements WHERE id=?`, id))
}
func (s *PluginStore) PutPlacement(ctx context.Context, id string, b plugins.PlacementBody, match string) (plugins.Placement, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return plugins.Placement{}, false, err
	}
	defer tx.Rollback()
	old, err := scanPlacement(tx.QueryRowContext(ctx, `SELECT `+placementColumns+` FROM plugin_placements WHERE id=?`, id))
	created := plugins.ErrorCode(err) == "not_found"
	if err != nil && !created {
		return old, false, err
	}
	if !created {
		if match == "" {
			if plugins.SamePlacement(old.PlacementBody, b) {
				return old, false, nil
			}
			return old, false, plugins.Fail("conflict")
		}
		if match != old.Revision {
			return old, false, plugins.Fail("precondition_failed")
		}
		if old.PluginID != b.PluginID || old.ConnectionID != b.ConnectionID || old.ContributionID != b.ContributionID {
			return old, false, plugins.Fail("conflict")
		}
		if plugins.SamePlacement(old.PlacementBody, b) {
			return old, false, nil
		}
	} else {
		if match != "" {
			return old, false, plugins.Fail("precondition_failed")
		}
		var count int
		err = tx.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM dashboard_selections)+(SELECT COUNT(*) FROM plugin_placements)`).Scan(&count)
		if err != nil {
			return old, false, err
		}
		if count >= 128 {
			return old, false, plugins.Fail("conflict")
		}
	}
	now := time.Now().UTC()
	raw := now.Format(time.RFC3339Nano)
	if created {
		_, err = tx.ExecContext(ctx, `INSERT INTO plugin_placements(`+placementColumns+`) VALUES(?,?,?,?,?,?,1,?,?)`, id, b.PluginID, b.ConnectionID, b.ContributionID, string(b.Options), b.Enabled, raw, raw)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE plugin_placements SET options=?,enabled=?,revision=revision+1,updated_at=? WHERE id=?`, string(b.Options), b.Enabled, raw, id)
	}
	if err != nil {
		return old, false, plugins.Fail("conflict")
	}
	value, err := scanPlacement(tx.QueryRowContext(ctx, `SELECT `+placementColumns+` FROM plugin_placements WHERE id=?`, id))
	if err != nil {
		return value, false, err
	}
	if err = tx.Commit(); err != nil {
		return value, false, err
	}
	return value, created, nil
}
func (s *PluginStore) DeletePlacement(ctx context.Context, id, match string) error {
	if match == "" {
		return plugins.Fail("precondition_required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	old, err := scanPlacement(tx.QueryRowContext(ctx, `SELECT `+placementColumns+` FROM plugin_placements WHERE id=?`, id))
	if err != nil {
		return err
	}
	if old.Revision != match {
		return plugins.Fail("precondition_failed")
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM plugin_placements WHERE id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}
