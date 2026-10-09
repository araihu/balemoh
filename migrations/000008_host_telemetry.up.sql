CREATE TABLE telemetry_hosts (
 id TEXT PRIMARY KEY,
 source_kind TEXT NOT NULL,
 source_id TEXT NOT NULL,
 node TEXT NOT NULL,
 observed_at INTEGER NOT NULL DEFAULT 0,
 received_at INTEGER NOT NULL DEFAULT 0,
 observation TEXT NOT NULL,
 UNIQUE(source_kind, source_id, node)
);
CREATE TABLE telemetry_samples (
 host_id TEXT NOT NULL REFERENCES telemetry_hosts(id) ON DELETE CASCADE,
 minute INTEGER NOT NULL,
 metrics TEXT NOT NULL,
 PRIMARY KEY(host_id, minute)
);
CREATE INDEX telemetry_samples_age ON telemetry_samples(minute);
