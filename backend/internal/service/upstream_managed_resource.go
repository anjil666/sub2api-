package service

import "time"

// UpstreamManagedResource 上游站点托管资源（每个 API Key 一条记录）
type UpstreamManagedResource struct {
	ID                    int64
	UpstreamSiteID        int64
	UpstreamKeyID         string  // 远程 key 的 ID（数字字符串）
	UpstreamKeyPrefix     string  // "sk-abc1..." 前缀用于显示
	UpstreamKeyName       string  // 远程 key 的名称
	UpstreamGroupID       *int64  // 远程 key 的 group_id
	APIKey                string  // 完整 sk-... （内存明文，存储时 AES 加密）
	ManagedGroupID        *int64
	ManagedAccountID      *int64
	ManagedChannelID      *int64
	PriceMultiplier       float64 // 0 = 使用站点默认倍率
	UpstreamRateMultiplier float64 // 上游分组的原始倍率（参考值）
	ModelCount            int
	Status                string // "active", "stale"
	LastSyncedAt          *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time

	// 非持久化字段（仅同步期间使用）
	UpstreamGroupDescription string `json:"-"` // 上游分组描述
}
