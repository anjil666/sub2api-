-- Migration 107: Add video studio support to groups
ALTER TABLE groups ADD COLUMN IF NOT EXISTS video_studio_enabled BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE groups ADD COLUMN IF NOT EXISTS video_price DECIMAL(20,8);
