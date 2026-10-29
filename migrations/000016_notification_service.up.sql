ALTER TABLE notifications ADD COLUMN service_id TEXT NOT NULL DEFAULT '' CHECK (service_id = '' OR (length(service_id) = 64 AND service_id NOT GLOB '*[^0-9a-f]*'));
