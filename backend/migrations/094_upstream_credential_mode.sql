-- 094: Upstream sites credential mode + managed resources table
-- Supports email+password login mode in addition to manual API key entry
-- Replaces single managed_group_id/account_id/channel_id with a 1:N child table

-- 1. Add credential mode and login fields to upstream_sites
ALTER TABLE upstream_sites
    ADD COLUMN IF NOT EXISTS credential_mode VARCHAR(20) NOT NULL DEFAULT 'api_key',
    ADD COLUMN IF NOT EXISTS email_encrypted TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS password_encrypted TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS cached_access_token TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS cached_refresh_token TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS token_expires_at TIMESTAMPTZ;

-- 2. Create managed resources table (1:N per upstream site)
CREATE TABLE IF NOT EXISTS upstream_managed_resources (
    id BIGSERIAL PRIMARY KEY,
    upstream_site_id BIGINT NOT NULL REFERENCES upstream_sites(id) ON DELETE CASCADE,
    upstream_key_id TEXT NOT NULL,
    upstream_key_prefix TEXT NOT NULL DEFAULT '',
    upstream_key_name TEXT NOT NULL DEFAULT '',
    upstream_group_id BIGINT,
    api_key_encrypted TEXT NOT NULL DEFAULT '',
    managed_group_id BIGINT,
    managed_account_id BIGINT,
    managed_channel_id BIGINT,
    model_count INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    last_synced_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (upstream_site_id, upstream_key_id)
);

CREATE INDEX IF NOT EXISTS idx_umr_site_id ON upstream_managed_resources(upstream_site_id);

-- 3. Migrate existing single-resource data into the new table
-- Only run if managed_group_id column still exists
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'upstream_sites' AND column_name = 'managed_group_id'
    ) THEN
        INSERT INTO upstream_managed_resources (
            upstream_site_id, upstream_key_id, upstream_key_prefix,
            api_key_encrypted, managed_group_id, managed_account_id, managed_channel_id,
            status, created_at, updated_at
        )
        SELECT id, 'legacy-' || id::TEXT, 'legacy',
               COALESCE(api_key_encrypted, ''), managed_group_id, managed_account_id, managed_channel_id,
               'active', created_at, COALESCE(updated_at, NOW())
        FROM upstream_sites
        WHERE managed_group_id IS NOT NULL OR managed_account_id IS NOT NULL OR managed_channel_id IS NOT NULL
        ON CONFLICT (upstream_site_id, upstream_key_id) DO NOTHING;

        -- 4. Drop old single-value columns
        ALTER TABLE upstream_sites DROP COLUMN IF EXISTS managed_group_id;
        ALTER TABLE upstream_sites DROP COLUMN IF EXISTS managed_account_id;
        ALTER TABLE upstream_sites DROP COLUMN IF EXISTS managed_channel_id;
    END IF;
END$$;

COMMENT ON COLUMN upstream_sites.credential_mode IS 'api_key = manual key, login = email+password auto-discover';
COMMENT ON TABLE upstream_managed_resources IS 'Per-API-key managed resources for upstream sites (1:N)';
