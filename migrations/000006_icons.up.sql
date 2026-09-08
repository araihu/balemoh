ALTER TABLE service_edits ADD COLUMN icon_ref TEXT NOT NULL DEFAULT '';
CREATE TABLE uploaded_icons (
 id TEXT PRIMARY KEY,
 name TEXT NOT NULL,
 tags TEXT NOT NULL DEFAULT '',
 mime TEXT NOT NULL,
 digest TEXT NOT NULL,
 data BLOB NOT NULL
);
CREATE INDEX service_edits_icon_ref ON service_edits(icon_ref);
