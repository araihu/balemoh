CREATE TABLE discovered_services (
    id TEXT PRIMARY KEY NOT NULL,
    source_kind TEXT NOT NULL,
    source_id TEXT NOT NULL,
    resource_kind TEXT NOT NULL,
    resource_namespace TEXT NOT NULL DEFAULT '',
    resource_name TEXT NOT NULL,
    display_name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    observed_at TEXT NOT NULL,
    pinned_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (source_kind, source_id, resource_kind, resource_namespace, resource_name)
);

CREATE INDEX discovered_services_pinned_idx
    ON discovered_services (pinned_at);

CREATE INDEX discovered_services_display_name_idx
    ON discovered_services (display_name, id);

CREATE TABLE service_endpoints (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    service_id TEXT NOT NULL,
    name TEXT NOT NULL,
    url TEXT NOT NULL DEFAULT '',
    port INTEGER NOT NULL DEFAULT 0,
    protocol TEXT NOT NULL DEFAULT '',
    provenance TEXT NOT NULL DEFAULT '',
    FOREIGN KEY (service_id) REFERENCES discovered_services (id) ON DELETE CASCADE
);

CREATE INDEX service_endpoints_service_idx
    ON service_endpoints (service_id);
