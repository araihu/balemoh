ALTER TABLE presentation_themes ADD COLUMN brand_logo_id TEXT REFERENCES uploaded_icons(id) ON DELETE RESTRICT;
UPDATE presentation_themes SET brand_logo_id=brand_icon_id;
CREATE INDEX presentation_themes_brand_logo ON presentation_themes(brand_logo_id);
