-- +migrate Up
ALTER TABLE upstream_managed_resources ADD COLUMN IF NOT EXISTS model_filter TEXT NOT NULL DEFAULT '';

-- +migrate Down
ALTER TABLE upstream_managed_resources DROP COLUMN IF EXISTS model_filter;
