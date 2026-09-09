package sqlite

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"github.com/araihu/balemoh/internal/application/icons"
	"strings"
)

type IconStore struct{ db *sql.DB }

func NewIconStore(db *sql.DB) *IconStore { return &IconStore{db} }
func (s *IconStore) List(ctx context.Context) ([]icons.Icon, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,name,tags,mime,digest FROM uploaded_icons ORDER BY name,id")
	if err != nil {
		return nil, err
	}
	result := []icons.Icon{}
	for rows.Next() {
		var i icons.Icon
		if err = rows.Scan(&i.ID, &i.Name, &i.Tags, &i.MIME, &i.Digest); err != nil {
			rows.Close()
			return nil, err
		}
		i.UsedBy = []string{}
		result = append(result, i)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for idx := range result {
		rows, err = s.db.QueryContext(ctx, "SELECT display_name FROM service_edits WHERE icon_ref=? ORDER BY display_name", result[idx].ID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var n string
			if err = rows.Scan(&n); err != nil {
				rows.Close()
				return nil, err
			}
			result[idx].UsedBy = append(result[idx].UsedBy, n)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
func (s *IconStore) Get(ctx context.Context, id string) (icons.Icon, error) {
	var i icons.Icon
	err := s.db.QueryRowContext(ctx, "SELECT id,name,tags,mime,digest,data FROM uploaded_icons WHERE id=?", id).Scan(&i.ID, &i.Name, &i.Tags, &i.MIME, &i.Digest, &i.Data)
	if errors.Is(err, sql.ErrNoRows) {
		err = icons.ErrNotFound
	}
	return i, err
}
func (s *IconStore) Save(ctx context.Context, id, name, tags, revision string, data []byte) (icons.Icon, error) {
	name, tags = strings.TrimSpace(name), strings.TrimSpace(tags)
	if id != "" && len(data) == 0 {
		old, err := s.Get(ctx, id)
		if err != nil {
			return icons.Icon{}, err
		}
		data = old.Data
	}
	mime, err := icons.Validate(name, tags, data)
	if err != nil {
		return icons.Icon{}, icons.ErrInvalid
	}
	sum := sha256.Sum256(append([]byte(name+"\x00"+tags+"\x00"), data...))
	digest := hex.EncodeToString(sum[:])
	if id == "" {
		id = "upload:" + rand.Text()
		_, err = s.db.ExecContext(ctx, "INSERT INTO uploaded_icons(id,name,tags,mime,digest,data) VALUES(?,?,?,?,?,?)", id, name, tags, mime, digest, data)
	} else {
		var res sql.Result
		res, err = s.db.ExecContext(ctx, "UPDATE uploaded_icons SET name=?,tags=?,mime=?,digest=?,data=? WHERE id=? AND digest=?", name, tags, mime, digest, data, id, revision)
		if err == nil {
			n, e := res.RowsAffected()
			if e != nil {
				return icons.Icon{}, e
			}
			if n != 1 {
				return icons.Icon{}, icons.ErrConflict
			}
		}
	}
	if err != nil {
		return icons.Icon{}, err
	}
	return icons.Icon{ID: id, Name: name, Tags: tags, MIME: mime, Digest: digest, UsedBy: []string{}}, nil
}
func (s *IconStore) Delete(ctx context.Context, id, revision string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM uploaded_icons WHERE id=? AND digest=? AND NOT EXISTS(SELECT 1 FROM service_edits WHERE icon_ref=?)", id, revision, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return icons.ErrConflict
	}
	return nil
}
