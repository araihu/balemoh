CREATE TABLE discovery_source_snapshots (
    source_kind TEXT NOT NULL,
    source_id TEXT NOT NULL,
    observed_at TEXT NOT NULL,
    PRIMARY KEY (source_kind, source_id)
);
