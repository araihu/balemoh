CREATE TABLE presentation_themes (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, schema_version INTEGER NOT NULL,
 definition_json TEXT NOT NULL, provenance_json TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision>=1), creation_digest TEXT NOT NULL
);
CREATE TABLE presentation_settings (
 id INTEGER PRIMARY KEY CHECK(id=1), builtin_key TEXT, custom_theme_id TEXT REFERENCES presentation_themes(id) ON DELETE RESTRICT,
 color_mode TEXT NOT NULL CHECK(color_mode IN ('system','light','dark')),
 revision INTEGER NOT NULL CHECK(revision>=0),
 last_command_id TEXT NOT NULL DEFAULT '', last_payload_digest TEXT NOT NULL DEFAULT '', last_receipt TEXT NOT NULL DEFAULT '',
 CHECK((builtin_key IS NULL) != (custom_theme_id IS NULL))
);
INSERT INTO presentation_settings(id,builtin_key,color_mode,revision) VALUES(1,'goshtoso','system',0);
CREATE TABLE presentation_fonts (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, family TEXT NOT NULL, weight TEXT NOT NULL, style TEXT NOT NULL,
 variable INTEGER NOT NULL, format TEXT NOT NULL, mime TEXT NOT NULL,
 byte_length INTEGER NOT NULL CHECK(byte_length BETWEEN 1 AND 10485760), digest TEXT NOT NULL, data BLOB NOT NULL,
 revision INTEGER NOT NULL CHECK(revision=1), creation_digest TEXT NOT NULL
);
CREATE TABLE presentation_theme_fonts (
 theme_id TEXT NOT NULL REFERENCES presentation_themes(id) ON DELETE CASCADE,
 role TEXT NOT NULL CHECK(role IN ('title','body')), font_id TEXT NOT NULL REFERENCES presentation_fonts(id) ON DELETE RESTRICT,
 PRIMARY KEY(theme_id,role)
);
CREATE INDEX presentation_theme_fonts_font ON presentation_theme_fonts(font_id);
