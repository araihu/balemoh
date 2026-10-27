package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	p "github.com/araihu/balemoh/internal/application/presentation"
	"github.com/araihu/goshtoso/themes"
)

type PresentationStore struct{ db *sql.DB }

func NewPresentationStore(db *sql.DB) *PresentationStore { return &PresentationStore{db} }

type presentationQuery interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func digest(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func jsonText(v any) string { b, _ := json.Marshal(v); return string(b) }
func themeRow(ctx context.Context, q presentationQuery, id string) (p.Theme, error) {
	var t p.Theme
	var data []byte
	var rev int64
	e := q.QueryRowContext(ctx, "SELECT id,name,revision,definition_json,coalesce(brand_icon_id,''),coalesce(brand_logo_id,'') FROM presentation_themes WHERE id=?", id).Scan(&t.ID, &t.Name, &rev, &data, &t.BrandIconID, &t.BrandLogoID)
	if errors.Is(e, sql.ErrNoRows) {
		return t, p.Missing()
	}
	if e != nil {
		return t, e
	}
	t.Revision = strconv.FormatInt(rev, 10)
	t.Definition, e = themes.Decode(data)
	return t, e
}
func settingsRow(ctx context.Context, q presentationQuery) (p.Settings, string, string, string, error) {
	var s p.Settings
	var builtin, custom sql.NullString
	var rev int64
	var command, payload, receipt string
	e := q.QueryRowContext(ctx, "SELECT builtin_key,custom_theme_id,color_mode,revision,last_command_id,last_payload_digest,last_receipt FROM presentation_settings WHERE id=1").Scan(&builtin, &custom, &s.ColorMode, &rev, &command, &payload, &receipt)
	s.Revision = strconv.FormatInt(rev, 10)
	if builtin.Valid {
		s.Selection = p.Selection{Kind: "builtin", Key: builtin.String}
	} else {
		s.Selection = p.Selection{Kind: "custom", ID: custom.String}
	}
	return s, command, payload, receipt, e
}
func (s *PresentationStore) State(ctx context.Context) (p.State, error) {
	tx, e := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return p.State{}, e
	}
	defer tx.Rollback()
	settings, _, _, _, e := settingsRow(ctx, tx)
	if e != nil {
		return p.State{}, e
	}
	var d themes.Definition
	var brand, logo string
	if settings.Selection.Kind == "builtin" {
		d, e = themes.Preset(themes.Key(settings.Selection.Key))
	} else {
		var t p.Theme
		t, e = themeRow(ctx, tx, settings.Selection.ID)
		d = t.Definition
		brand = t.BrandIconID
		logo = t.BrandLogoID
		settings.Selection.ExpectedThemeRevision = t.Revision
	}
	if e != nil {
		return p.State{}, e
	}
	if e = checkFonts(ctx, tx, d); e != nil {
		return p.State{}, e
	}
	if e = tx.Commit(); e != nil {
		return p.State{}, e
	}
	return p.State{BrandIconID: brand, BrandLogoID: logo, Settings: settings, CatalogRevision: themes.CatalogRevision(), Definition: d}, nil
}
func (s *PresentationStore) GetTheme(ctx context.Context, id string) (p.Theme, error) {
	return themeRow(ctx, s.db, id)
}
func (s *PresentationStore) Catalog(ctx context.Context) (p.Catalog, error) {
	b, e := p.Builtins()
	if e != nil {
		return p.Catalog{}, e
	}
	out := p.Catalog{CatalogRevision: themes.CatalogRevision(), Builtins: b, Custom: []p.Theme{}}
	rows, e := s.db.QueryContext(ctx, "SELECT id,name,revision,definition_json,coalesce(brand_icon_id,''),coalesce(brand_logo_id,'') FROM presentation_themes ORDER BY name,id")
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var t p.Theme
		var data []byte
		var rev int64
		if e = rows.Scan(&t.ID, &t.Name, &rev, &data, &t.BrandIconID, &t.BrandLogoID); e != nil {
			return out, e
		}
		t.Revision = strconv.FormatInt(rev, 10)
		t.Definition, e = themes.Decode(data)
		if e != nil {
			return out, e
		}
		out.Custom = append(out.Custom, t)
	}
	return out, rows.Err()
}
func checkFonts(ctx context.Context, q presentationQuery, d themes.Definition) error {
	if e := themes.Validate(d); e != nil {
		return p.Invalid()
	}
	for _, ref := range []themes.FontRef{d.Typography.Title, d.Typography.Body} {
		if ref.Source == "upload" {
			var id string
			if !p.ValidID(ref.ID, "font") {
				return p.Invalid()
			}
			e := q.QueryRowContext(ctx, "SELECT id FROM presentation_fonts WHERE id=?", ref.ID).Scan(&id)
			if errors.Is(e, sql.ErrNoRows) {
				return p.Invalid()
			}
			if e != nil {
				return e
			}
		} else if _, e := themes.DefaultFontResolver(ref); e != nil {
			return p.Invalid()
		}
	}
	return nil
}
func references(ctx context.Context, q presentationQuery, id string, d themes.Definition) error {
	if _, e := q.ExecContext(ctx, "DELETE FROM presentation_theme_fonts WHERE theme_id=?", id); e != nil {
		return e
	}
	for role, ref := range map[string]themes.FontRef{"title": d.Typography.Title, "body": d.Typography.Body} {
		if ref.Source == "upload" {
			if _, e := q.ExecContext(ctx, "INSERT INTO presentation_theme_fonts(theme_id,role,font_id) VALUES(?,?,?)", id, role, ref.ID); e != nil {
				return e
			}
		}
	}
	return nil
}
func insertTheme(ctx context.Context, q presentationQuery, t p.Theme, creationDigest string) error {
	var count int
	if e := q.QueryRowContext(ctx, "SELECT count(*) FROM presentation_themes").Scan(&count); e != nil {
		return e
	}
	if count >= p.MaxThemes {
		return &p.Error{Code: "limit_exceeded", Message: "Theme catalog is full."}
	}
	if _, e := q.ExecContext(ctx, "INSERT INTO presentation_themes(id,name,schema_version,definition_json,provenance_json,revision,creation_digest,brand_icon_id,brand_logo_id) VALUES(?,?,?,?,?,1,?,nullif(?,''),nullif(?,''))", t.ID, t.Name, t.Definition.SchemaVersion, jsonText(t.Definition), jsonText(t.Definition.Provenance), creationDigest, t.BrandIconID, t.BrandLogoID); e != nil {
		return e
	}
	return references(ctx, q, t.ID, t.Definition)
}
func (s *PresentationStore) Save(ctx context.Context, c p.Save) (p.Receipt, error) {
	if !p.ValidCommand(c.CommandID) || !p.ValidRevision(c.ExpectedSettingsRevision) || (c.ColorMode != "system" && c.ColorMode != "light" && c.ColorMode != "dark") {
		return p.Receipt{}, p.Invalid()
	}
	v := c.Selection
	switch v.Kind {
	case "builtin":
		if v.BrandLogoID != nil || v.BrandIconID != "" || v.ID != "" || v.Name != "" || v.Definition != nil || v.Draft != nil || v.ExpectedThemeRevision != "" || v.ExpectedCatalogRevision == "" {
			return p.Receipt{}, p.Invalid()
		}
	case "custom":
		if v.BrandLogoID != nil || v.BrandIconID != "" || !p.ValidID(v.ID, "theme") || !p.ValidRevision(v.ExpectedThemeRevision) || v.Key != "" || v.Definition != nil || v.Name != "" || v.ExpectedCatalogRevision != "" {
			return p.Receipt{}, p.Invalid()
		}
	case "newCustom":
		if !p.ValidID(v.ID, "theme") || !p.ValidName(v.Name) || v.Definition == nil || v.ExpectedCatalogRevision == "" || v.Key != "" || v.Draft != nil || v.ExpectedThemeRevision != "" {
			return p.Receipt{}, p.Invalid()
		}
	default:
		return p.Receipt{}, p.Invalid()
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return p.Receipt{}, e
	}
	defer tx.Rollback()
	settings, lastID, lastDigest, lastReceipt, e := settingsRow(ctx, tx)
	if e != nil {
		return p.Receipt{}, e
	}
	payload := digest(c)
	if lastID == c.CommandID {
		if lastDigest != payload {
			return p.Receipt{}, p.Stale("command", c.CommandID, settings.Revision)
		}
		var result p.Receipt
		if e = json.Unmarshal([]byte(lastReceipt), &result); e != nil {
			return result, e
		}
		return result, nil
	}
	if settings.Revision != c.ExpectedSettingsRevision {
		return p.Receipt{}, p.Stale("settings", "", settings.Revision)
	}
	if v.ExpectedCatalogRevision != "" && v.ExpectedCatalogRevision != themes.CatalogRevision() {
		return p.Receipt{}, p.Stale("catalog", "", themes.CatalogRevision())
	}
	var selected p.Selection
	var themeRevision string
	switch v.Kind {
	case "builtin":
		d, err := themes.Preset(themes.Key(v.Key))
		if err != nil {
			return p.Receipt{}, p.Invalid()
		}
		if err = checkFonts(ctx, tx, d); err != nil {
			return p.Receipt{}, err
		}
		selected = p.Selection{Kind: "builtin", Key: v.Key}
	case "custom":
		t, err := themeRow(ctx, tx, v.ID)
		if err != nil {
			return p.Receipt{}, err
		}
		if t.Revision != v.ExpectedThemeRevision {
			return p.Receipt{}, p.Stale("theme", v.ID, t.Revision)
		}
		if v.Draft != nil {
			if v.Draft.BrandIconID != nil {
				t.BrandIconID = *v.Draft.BrandIconID
			}
			if v.Draft.BrandLogoID != nil {
				t.BrandLogoID = *v.Draft.BrandLogoID
			}
			if err = checkBrand(ctx, tx, t.BrandLogoID); err != nil {
				return p.Receipt{}, err
			}
			if err = checkBrand(ctx, tx, t.BrandIconID); err != nil {
				return p.Receipt{}, err
			}
			if !p.ValidName(v.Draft.Name) {
				return p.Receipt{}, p.Invalid()
			}
			if err = checkFonts(ctx, tx, v.Draft.Definition); err != nil {
				return p.Receipt{}, err
			}
			if _, err = tx.ExecContext(ctx, "UPDATE presentation_themes SET name=?,definition_json=?,provenance_json=?,brand_icon_id=nullif(?,''),brand_logo_id=nullif(?,''),revision=revision+1 WHERE id=?", v.Draft.Name, jsonText(v.Draft.Definition), jsonText(v.Draft.Definition.Provenance), t.BrandIconID, t.BrandLogoID, v.ID); err != nil {
				return p.Receipt{}, err
			}
			if err = references(ctx, tx, v.ID, v.Draft.Definition); err != nil {
				return p.Receipt{}, err
			}
			n, _ := strconv.ParseInt(t.Revision, 10, 64)
			t.Revision = strconv.FormatInt(n+1, 10)
		} else if err = checkFonts(ctx, tx, t.Definition); err != nil {
			return p.Receipt{}, err
		}
		themeRevision = t.Revision
		selected = p.Selection{Kind: "custom", ID: v.ID}
	case "newCustom":
		logo := v.BrandIconID
		if v.BrandLogoID != nil {
			logo = *v.BrandLogoID
		}
		if err := checkBrand(ctx, tx, logo); err != nil {
			return p.Receipt{}, err
		}
		if err := checkBrand(ctx, tx, v.BrandIconID); err != nil {
			return p.Receipt{}, err
		}
		if err := checkFonts(ctx, tx, *v.Definition); err != nil {
			return p.Receipt{}, err
		}
		var existing string
		err := tx.QueryRowContext(ctx, "SELECT id FROM presentation_themes WHERE id=?", v.ID).Scan(&existing)
		if err == nil {
			return p.Receipt{}, p.Stale("theme", v.ID, "")
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return p.Receipt{}, err
		}
		if err = insertTheme(ctx, tx, p.Theme{ID: v.ID, Name: v.Name, Definition: *v.Definition, BrandIconID: v.BrandIconID, BrandLogoID: logo}, digest(p.Create{ID: v.ID, Name: v.Name, Definition: v.Definition, BrandIconID: v.BrandIconID, BrandLogoID: v.BrandLogoID})); err != nil {
			return p.Receipt{}, err
		}
		selected = p.Selection{Kind: "custom", ID: v.ID}
		themeRevision = "1"
	}
	rev, _ := strconv.ParseInt(settings.Revision, 10, 64)
	result := p.Receipt{CommandID: c.CommandID, Settings: p.Settings{Revision: strconv.FormatInt(rev+1, 10), ColorMode: c.ColorMode, Selection: selected}, ThemeRevision: themeRevision}
	var builtin, custom any
	if selected.Kind == "builtin" {
		builtin = selected.Key
	} else {
		custom = selected.ID
	}
	if _, e = tx.ExecContext(ctx, "UPDATE presentation_settings SET builtin_key=?,custom_theme_id=?,color_mode=?,revision=revision+1,last_command_id=?,last_payload_digest=?,last_receipt=? WHERE id=1", builtin, custom, c.ColorMode, c.CommandID, payload, jsonText(result)); e != nil {
		return p.Receipt{}, e
	}
	if e = tx.Commit(); e != nil {
		return p.Receipt{}, e
	}
	return result, nil
}
func (s *PresentationStore) CreateTheme(ctx context.Context, c p.Create) (p.Theme, bool, error) {
	if !p.ValidID(c.ID, "theme") || !p.ValidName(c.Name) || (c.Definition == nil) == (c.CopyFrom == nil) || c.CopyFrom != nil && (c.BrandIconID != "" || c.BrandLogoID != nil) {
		return p.Theme{}, false, p.Invalid()
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return p.Theme{}, false, e
	}
	defer tx.Rollback()
	var stored string
	e = tx.QueryRowContext(ctx, "SELECT creation_digest FROM presentation_themes WHERE id=?", c.ID).Scan(&stored)
	if e == nil {
		if stored != digest(c) {
			return p.Theme{}, false, p.Stale("theme", c.ID, "")
		}
		t, err := themeRow(ctx, tx, c.ID)
		return t, false, err
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return p.Theme{}, false, e
	}
	var d themes.Definition
	brand := c.BrandIconID
	logo := brand
	if c.BrandLogoID != nil {
		logo = *c.BrandLogoID
	}
	if c.CopyFrom != nil {
		if !p.ValidID(c.CopyFrom.ID, "theme") || !p.ValidRevision(c.CopyFrom.ExpectedRevision) {
			return p.Theme{}, false, p.Invalid()
		}
		t, err := themeRow(ctx, tx, c.CopyFrom.ID)
		if err != nil {
			return p.Theme{}, false, err
		}
		if t.Revision != c.CopyFrom.ExpectedRevision {
			return p.Theme{}, false, p.Stale("theme", t.ID, t.Revision)
		}
		d = t.Definition.Clone()
		brand = t.BrandIconID
		logo = t.BrandLogoID
	} else {
		d = c.Definition.Clone()
	}
	if e = checkFonts(ctx, tx, d); e != nil {
		return p.Theme{}, false, e
	}
	if e = checkBrand(ctx, tx, logo); e != nil {
		return p.Theme{}, false, e
	}
	if e = checkBrand(ctx, tx, brand); e != nil {
		return p.Theme{}, false, e
	}
	t := p.Theme{ID: c.ID, Name: c.Name, Revision: "1", Definition: d, BrandIconID: brand, BrandLogoID: logo}
	if e = insertTheme(ctx, tx, t, digest(c)); e != nil {
		return t, false, e
	}
	e = tx.Commit()
	return t, true, e
}
func (s *PresentationStore) RenameTheme(ctx context.Context, id string, c p.Rename) (p.Theme, error) {
	if !p.ValidID(id, "theme") || !p.ValidName(c.Name) || !p.ValidRevision(c.ExpectedRevision) {
		return p.Theme{}, p.Invalid()
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return p.Theme{}, e
	}
	defer tx.Rollback()
	t, e := themeRow(ctx, tx, id)
	if e != nil {
		return t, e
	}
	if t.Revision != c.ExpectedRevision {
		return t, p.Stale("theme", id, t.Revision)
	}
	if _, e = tx.ExecContext(ctx, "UPDATE presentation_themes SET name=?,revision=revision+1 WHERE id=?", c.Name, id); e != nil {
		return t, e
	}
	n, _ := strconv.ParseInt(t.Revision, 10, 64)
	t.Name = c.Name
	t.Revision = strconv.FormatInt(n+1, 10)
	return t, tx.Commit()
}
func (s *PresentationStore) DeleteTheme(ctx context.Context, id, revision string) error {
	if !p.ValidID(id, "theme") || !p.ValidRevision(revision) {
		return p.Invalid()
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	t, e := themeRow(ctx, tx, id)
	if e != nil {
		return e
	}
	if t.Revision != revision {
		return p.Stale("theme", id, t.Revision)
	}
	settings, _, _, _, e := settingsRow(ctx, tx)
	if e != nil {
		return e
	}
	if settings.Selection.ID == id {
		return &p.Error{Code: "in_use", Message: "Select and save another theme before deleting this theme.", UsedBy: []string{"presentation"}}
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM presentation_themes WHERE id=?", id); e != nil {
		return e
	}
	return tx.Commit()
}
func fontRow(ctx context.Context, q presentationQuery, id string, bytes bool) (p.Font, error) {
	var f p.Font
	columns := "id,name,family,weight,style,variable,format,mime,byte_length,digest,revision"
	args := []any{&f.ID, &f.Name, &f.Family, &f.Weight, &f.Style, &f.Variable, &f.Format, &f.MIME, &f.ByteLength, &f.Digest, &f.Revision}
	if bytes {
		columns += ",data"
		args = append(args, &f.Data)
	}
	e := q.QueryRowContext(ctx, "SELECT "+columns+" FROM presentation_fonts WHERE id=?", id).Scan(args...)
	if errors.Is(e, sql.ErrNoRows) {
		e = p.Missing()
	}
	f.UsedBy = []string{}
	return f, e
}
func (s *PresentationStore) Font(ctx context.Context, id string) (p.Font, error) {
	return fontRow(ctx, s.db, id, true)
}
func (s *PresentationStore) Fonts(ctx context.Context) ([]p.Font, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT id,name,family,weight,style,variable,format,mime,byte_length,digest,revision FROM presentation_fonts ORDER BY name,id")
	if e != nil {
		return nil, e
	}
	out := []p.Font{}
	for rows.Next() {
		var f p.Font
		if e = rows.Scan(&f.ID, &f.Name, &f.Family, &f.Weight, &f.Style, &f.Variable, &f.Format, &f.MIME, &f.ByteLength, &f.Digest, &f.Revision); e != nil {
			rows.Close()
			return nil, e
		}
		f.UsedBy = []string{}
		out = append(out, f)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	for i := range out {
		rows, e = s.db.QueryContext(ctx, "SELECT DISTINCT t.name || ' [' || t.id || ']' FROM presentation_theme_fonts f JOIN presentation_themes t ON t.id=f.theme_id WHERE f.font_id=? ORDER BY t.name,t.id", out[i].ID)
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			var name string
			if e = rows.Scan(&name); e != nil {
				rows.Close()
				return nil, e
			}
			out[i].UsedBy = append(out[i].UsedBy, name)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
	}
	return out, nil
}
func (s *PresentationStore) CreateFont(ctx context.Context, f p.Font) (p.Font, bool, error) {
	if !p.ValidID(f.ID, "font") || !p.ValidName(f.Name) || len(f.Data) == 0 || len(f.Data) > p.MaxFontBytes {
		return p.Font{}, false, p.Invalid()
	}
	sum := sha256.Sum256(f.Data)
	f.Digest = hex.EncodeToString(sum[:])
	f.ByteLength = len(f.Data)
	f.Revision = "1"
	f.UsedBy = []string{}
	creation := digest(struct{ ID, Name, Digest string }{f.ID, f.Name, f.Digest})
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return f, false, e
	}
	defer tx.Rollback()
	var stored string
	e = tx.QueryRowContext(ctx, "SELECT creation_digest FROM presentation_fonts WHERE id=?", f.ID).Scan(&stored)
	if e == nil {
		if stored != creation {
			return f, false, p.Stale("font", f.ID, "1")
		}
		old, err := fontRow(ctx, tx, f.ID, false)
		return old, false, err
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return f, false, e
	}
	var count, total int64
	if e = tx.QueryRowContext(ctx, "SELECT count(*),coalesce(sum(byte_length),0) FROM presentation_fonts").Scan(&count, &total); e != nil {
		return f, false, e
	}
	if count >= p.MaxFonts || total+int64(f.ByteLength) > p.MaxFontStorage {
		return f, false, &p.Error{Code: "limit_exceeded", Message: "Font storage limit reached."}
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO presentation_fonts(id,name,family,weight,style,variable,format,mime,byte_length,digest,data,revision,creation_digest) VALUES(?,?,?,?,?,?,?,?,?,?,?,1,?)", f.ID, f.Name, f.Family, f.Weight, f.Style, f.Variable, f.Format, f.MIME, f.ByteLength, f.Digest, f.Data, creation); e != nil {
		return f, false, e
	}
	if e = tx.Commit(); e != nil {
		return f, false, e
	}
	f.Data = nil
	return f, true, nil
}
func (s *PresentationStore) DeleteFont(ctx context.Context, id, revision string) error {
	if !p.ValidID(id, "font") || !p.ValidRevision(revision) {
		return p.Invalid()
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	f, e := fontRow(ctx, tx, id, false)
	if e != nil {
		return e
	}
	if f.Revision != revision {
		return p.Stale("font", id, f.Revision)
	}
	rows, e := tx.QueryContext(ctx, "SELECT DISTINCT t.name || ' [' || t.id || ']' FROM presentation_theme_fonts f JOIN presentation_themes t ON t.id=f.theme_id WHERE f.font_id=? ORDER BY t.name,t.id", id)
	if e != nil {
		return e
	}
	used := []string{}
	for rows.Next() {
		var n string
		if e = rows.Scan(&n); e != nil {
			rows.Close()
			return e
		}
		used = append(used, n)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	if len(used) > 0 {
		return &p.Error{Code: "in_use", Message: "Saved themes reference this font.", UsedBy: used}
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM presentation_fonts WHERE id=?", id); e != nil {
		return e
	}
	return tx.Commit()
}

var _ p.Store = (*PresentationStore)(nil)

func checkBrand(ctx context.Context, q presentationQuery, id string) error {
	if id == "" {
		return nil
	}
	if !strings.HasPrefix(id, "upload:") || len(id) > 200 {
		return p.Invalid()
	}
	var stored string
	err := q.QueryRowContext(ctx, "SELECT id FROM uploaded_icons WHERE id=?", id).Scan(&stored)
	if errors.Is(err, sql.ErrNoRows) {
		return p.Invalid()
	}
	return err
}
