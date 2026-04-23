-- +migrate Up
ALTER TABLE upstream_sites ADD COLUMN IF NOT EXISTS site_type VARCHAR(20) NOT NULL DEFAULT 'standard';

-- +migrate Down
ALTER TABLE upstream_sites DROP COLUMN IF EXISTS site_type;
