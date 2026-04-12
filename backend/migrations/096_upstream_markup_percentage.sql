-- 096: 将 price_multiplier 语义从"绝对倍率"改为"加价百分比"
-- 旧值 1.0（不加价）→ 新值 0（加价 0%）
-- 旧值 1.3（1.3x 倍率）→ 新值 30（加价 30%）

-- upstream_sites 表
UPDATE upstream_sites SET price_multiplier = ROUND((price_multiplier - 1) * 100, 2) WHERE price_multiplier != 1;
UPDATE upstream_sites SET price_multiplier = 0 WHERE price_multiplier = 1;
ALTER TABLE upstream_sites ALTER COLUMN price_multiplier SET DEFAULT 0;

-- upstream_managed_resources 表（0 仍然表示"继承站点默认"）
UPDATE upstream_managed_resources SET price_multiplier = ROUND((price_multiplier - 1) * 100, 2) WHERE price_multiplier > 0 AND price_multiplier != 1;
UPDATE upstream_managed_resources SET price_multiplier = 0 WHERE price_multiplier = 1;
