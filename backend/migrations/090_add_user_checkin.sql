-- 为 users 表添加签到字段
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS last_checkin_at TIMESTAMPTZ DEFAULT NULL;

COMMENT ON COLUMN users.last_checkin_at IS '最后签到时间';
