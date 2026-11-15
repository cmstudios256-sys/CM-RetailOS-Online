ALTER TABLE shops ADD COLUMN IF NOT EXISTS logo TEXT NOT NULL DEFAULT '';

UPDATE shops
SET logo = COALESCE(preferences->>'logo', '')
WHERE logo = '' AND COALESCE(preferences->>'logo', '') <> '';
