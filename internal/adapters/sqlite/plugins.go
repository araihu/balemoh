package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"
	"uuid"

	"github.com/araihu/balemoh/internal/application/plugins"
)

type PluginStore struct{ db *sql.DB }

func NewPluginStore(db *sql.DB) *PluginStore { return &PluginStore{db} }

const pluginColumns = `id,plugin_id,deployment_key,name,revision,config_digest,manifest_digest,plugin_version,enabled`

type pluginScanner interface{ Scan(...any) error }

func scanPlugin(row pluginScanner) (plugins.Connection, error) {
	var c plugins.Connection
	var revision int64
	err := row.Scan(&c.ID, &c.PluginID, &c.Key, &c.Name, &revision, &c.Digest, &c.ManifestDigest, &c.Version, &c.Enabled)
	c.Revision = strconv.FormatInt(revision, 10)
	if errors.Is(err, sql.ErrNoRows) {
		err = plugins.Fail("not_found")
	}
	return c, err
}
func (s *PluginStore) Get(ctx context.Context, plugin, id string) (plugins.Connection, error) {
	return scanPlugin(s.db.QueryRowContext(ctx, `SELECT `+pluginColumns+` FROM plugin_connections WHERE plugin_id=? AND id=?`, plugin, id))
}
func (s *PluginStore) List(ctx context.Context) ([]plugins.Connection, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+pluginColumns+` FROM plugin_connections ORDER BY plugin_id,deployment_key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []plugins.Connection{}
	for rows.Next() {
		c, err := scanPlugin(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}
func (s *PluginStore) Reconcile(ctx context.Context, items []plugins.Provision) ([]plugins.Connection, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	desired := map[string]bool{}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, p := range items {
		key := p.PluginID + "/" + p.Key
		if desired[key] || p.PluginID == "" || p.Key == "" || p.Name == "" || p.Digest == "" {
			return nil, plugins.Fail("invalid_argument")
		}
		desired[key] = true
		var id, digest, name string
		var revision int64
		var enabled bool
		err = tx.QueryRowContext(ctx, `SELECT id,config_digest,name,revision,enabled FROM plugin_connections WHERE plugin_id=? AND deployment_key=?`, p.PluginID, p.Key).Scan(&id, &digest, &name, &revision, &enabled)
		if errors.Is(err, sql.ErrNoRows) {
			id = uuid.NewV4().String()
			_, err = tx.ExecContext(ctx, `INSERT INTO plugin_connections(id,plugin_id,deployment_key,name,revision,config_digest,manifest_digest,plugin_version,enabled,created_at,updated_at) VALUES(?,?,?,?,1,?,?,?,1,?,?)`, id, p.PluginID, p.Key, p.Name, p.Digest, p.ManifestDigest, p.Version, now, now)
		} else if err == nil && (digest != p.Digest || name != p.Name || !enabled) {
			_, err = tx.ExecContext(ctx, `UPDATE plugin_connections SET name=?,revision=revision+1,config_digest=?,manifest_digest=?,plugin_version=?,enabled=1,updated_at=? WHERE id=?`, p.Name, p.Digest, p.ManifestDigest, p.Version, now, id)
		}
		if err != nil {
			return nil, err
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,plugin_id,deployment_key FROM plugin_connections WHERE enabled=1`)
	if err != nil {
		return nil, err
	}
	var omitted []string
	for rows.Next() {
		var id, plugin, key string
		if err := rows.Scan(&id, &plugin, &key); err != nil {
			rows.Close()
			return nil, err
		}
		if !desired[plugin+"/"+key] {
			omitted = append(omitted, id)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, id := range omitted {
		if _, err = tx.ExecContext(ctx, `UPDATE plugin_connections SET enabled=0,revision=revision+1,updated_at=? WHERE id=?`, now, id); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.List(ctx)
}
func (s *PluginStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM plugin_connections WHERE id=?`, id)
	// Placement foreign keys introduced by P3 must remain restrictive.
	if err != nil {
		return plugins.Fail("conflict")
	}
	return nil
}
