ALTER TABLE discovered_services ADD COLUMN missing INTEGER NOT NULL DEFAULT 0 CHECK (missing IN (0, 1));
ALTER TABLE discovered_services ADD COLUMN hidden INTEGER NOT NULL DEFAULT 0 CHECK (hidden IN (0, 1));
