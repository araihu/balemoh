CREATE TABLE IF NOT EXISTS plugin_connections (
 id TEXT PRIMARY KEY,
 plugin_id TEXT NOT NULL,
 deployment_key TEXT NOT NULL,
 name TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK (revision > 0),
 config_digest TEXT NOT NULL,
 manifest_digest TEXT NOT NULL,
 plugin_version TEXT NOT NULL,
 enabled INTEGER NOT NULL CHECK (enabled IN (0,1)),
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 UNIQUE (plugin_id, deployment_key)
);
-- Future placement records reference id with ON DELETE RESTRICT.
