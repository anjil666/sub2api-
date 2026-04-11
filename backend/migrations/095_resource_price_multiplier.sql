-- 为每个托管资源增加独立倍率字段
ALTER TABLE upstream_managed_resources
    ADD COLUMN IF NOT EXISTS price_multiplier DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS upstream_rate_multiplier DOUBLE PRECISION NOT NULL DEFAULT 0;

-- price_multiplier = 0 表示使用站点默认倍率
-- upstream_rate_multiplier 记录上游分组的原始倍率（仅展示参考用）
