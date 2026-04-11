-- 推荐返利系统：用户推荐码 + 返利记录表

-- 1. 为 users 表添加推荐码和推荐人字段
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS referral_code VARCHAR(16) DEFAULT NULL,
  ADD COLUMN IF NOT EXISTS referrer_id BIGINT DEFAULT NULL;

-- 推荐码唯一索引（排除 NULL）
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_referral_code
  ON users (referral_code) WHERE referral_code IS NOT NULL;

-- 推荐人索引（便于查询某人推荐了哪些用户）
CREATE INDEX IF NOT EXISTS idx_users_referrer_id
  ON users (referrer_id) WHERE referrer_id IS NOT NULL;

COMMENT ON COLUMN users.referral_code IS '推荐码（唯一，8位字母数字）';
COMMENT ON COLUMN users.referrer_id IS '推荐人用户ID';

-- 2. 创建返利记录表
CREATE TABLE IF NOT EXISTS referral_commissions (
  id              BIGSERIAL PRIMARY KEY,
  referrer_id     BIGINT NOT NULL,
  referred_id     BIGINT NOT NULL,
  order_code      VARCHAR(64) NOT NULL,
  order_amount    DECIMAL(20,8) NOT NULL DEFAULT 0,
  commission_rate DECIMAL(5,4) NOT NULL DEFAULT 0,
  commission_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
  status          VARCHAR(20) NOT NULL DEFAULT 'completed',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- order_code 唯一约束（保证幂等）
CREATE UNIQUE INDEX IF NOT EXISTS idx_referral_commissions_order_code
  ON referral_commissions (order_code);

-- 按推荐人查询索引
CREATE INDEX IF NOT EXISTS idx_referral_commissions_referrer_id
  ON referral_commissions (referrer_id);

-- 按被推荐人查询索引
CREATE INDEX IF NOT EXISTS idx_referral_commissions_referred_id
  ON referral_commissions (referred_id);

COMMENT ON TABLE referral_commissions IS '推荐返利记录';
COMMENT ON COLUMN referral_commissions.referrer_id IS '推荐人用户ID';
COMMENT ON COLUMN referral_commissions.referred_id IS '被推荐人用户ID';
COMMENT ON COLUMN referral_commissions.order_code IS '关联订单号（唯一，幂等）';
COMMENT ON COLUMN referral_commissions.order_amount IS '订单金额';
COMMENT ON COLUMN referral_commissions.commission_rate IS '返利比例（如 0.10 表示 10%）';
COMMENT ON COLUMN referral_commissions.commission_amount IS '返利金额';
COMMENT ON COLUMN referral_commissions.status IS '状态: completed';
