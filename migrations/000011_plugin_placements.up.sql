CREATE TABLE IF NOT EXISTS plugin_placements (
 id TEXT PRIMARY KEY,
 plugin_id TEXT NOT NULL,
 connection_id TEXT NOT NULL REFERENCES plugin_connections(id) ON DELETE RESTRICT,
 contribution_id TEXT NOT NULL,
 options TEXT NOT NULL,
 enabled INTEGER NOT NULL CHECK(enabled IN (0,1)),
 revision INTEGER NOT NULL CHECK(revision > 0),
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
