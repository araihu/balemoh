ALTER TABLE discovered_services
    ADD COLUMN images_json TEXT NOT NULL DEFAULT '[]';
