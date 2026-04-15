-- Add disabled_by column to distinguish auto-disabled (upstream removed) from manually disabled
ALTER TABLE upstream_managed_resources ADD COLUMN IF NOT EXISTS disabled_by TEXT NOT NULL DEFAULT '';
