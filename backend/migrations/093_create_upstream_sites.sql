-- 093: Create upstream_sites table for upstream sync management
-- Allows admins to manage upstream Sub2API sites and auto-sync groups/accounts/channels

CREATE TABLE IF NOT EXISTS upstream_sites (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL,
    platform VARCHAR(50) NOT NULL DEFAULT 'sub2api',
    base_url VARCHAR(500) NOT NULL,
    api_key_encrypted TEXT NOT NULL DEFAULT '',
    price_multiplier DECIMAL(10,4) NOT NULL DEFAULT 1.0000,
    sync_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    sync_interval_minutes INTEGER NOT NULL DEFAULT 60,
    last_sync_at TIMESTAMPTZ,
    last_sync_status VARCHAR(20) NOT NULL DEFAULT 'pending',
    last_sync_error TEXT NOT NULL DEFAULT '',
    last_sync_model_count INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    managed_group_id BIGINT,
    managed_account_id BIGINT,
    managed_channel_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_upstream_sites_base_url ON upstream_sites(base_url);
CREATE INDEX IF NOT EXISTS idx_upstream_sites_sync_due ON upstream_sites(sync_enabled, status, last_sync_at);

COMMENT ON TABLE upstream_sites IS '上游站点配置，用于自动同步上游 Sub2API 的分组、模型和账号';
COMMENT ON COLUMN upstream_sites.api_key_encrypted IS 'AES-GCM 加密存储的上游 API Key';
COMMENT ON COLUMN upstream_sites.price_multiplier IS '加价倍率，如 1.5 表示上游价格的 1.5 倍';
COMMENT ON COLUMN upstream_sites.managed_group_id IS '幂等标记：自动创建的分组 ID';
COMMENT ON COLUMN upstream_sites.managed_account_id IS '幂等标记：自动创建的账号 ID';
COMMENT ON COLUMN upstream_sites.managed_channel_id IS '幂等标记：自动创建的渠道 ID';
