CREATE TABLE service_edits (
 service_id TEXT PRIMARY KEY REFERENCES discovered_services(id) ON DELETE CASCADE,
 display_name TEXT NOT NULL,
 description TEXT NOT NULL,
 address TEXT NOT NULL
);
