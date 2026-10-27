ALTER TABLE presentation_themes ADD COLUMN brand_icon_id TEXT REFERENCES uploaded_icons(id) ON DELETE RESTRICT;
CREATE INDEX presentation_themes_brand_icon ON presentation_themes(brand_icon_id);
