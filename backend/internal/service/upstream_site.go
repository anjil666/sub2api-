package service

import "time"

// UpstreamSite 上游站点配置
type UpstreamSite struct {
	ID                  int64
	Name                string
	Platform            string // "sub2api"
	BaseURL             string
	APIKey              string // 内存中明文，存储时 AES 加密（api_key 模式）
	CredentialMode      string // "api_key" 或 "login"
	Email               string // login 模式邮箱（内存明文）
	Password            string // login 模式密码（内存明文）
	CachedAccessToken   string // 缓存的 JWT access token
	CachedRefreshToken  string // 缓存的 refresh token
	TokenExpiresAt      *time.Time
	PriceMultiplier     float64
	SyncEnabled         bool
	SyncIntervalMinutes int
	LastSyncAt          *time.Time
	LastSyncStatus      string // "pending", "success", "error"
	LastSyncError       string
	LastSyncModelCount  int
	Status              string // "active", "disabled"
	SiteType            string // "standard", "grsai"
	ManagedResourceCount int   // 从子表 COUNT 获取
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// UpstreamModelInfo 上游模型信息（来自 /v1/models）
type UpstreamModelInfo struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	DisplayName string `json:"display_name"`
}

// UpstreamBalanceInfo 上游余额信息（来自 /v1/usage）
type UpstreamBalanceInfo struct {
	BalanceUSD   float64 `json:"balance_usd"`
	UsedUSD      float64 `json:"used_usd"`
	RemainingUSD float64 `json:"remaining_usd"`
}

// SyncResult 同步结果
type SyncResult struct {
	ModelsDiscovered int    `json:"models_discovered"`
	KeysDiscovered   int    `json:"keys_discovered,omitempty"`
	GroupID          int64  `json:"group_id,omitempty"`
	AccountID        int64  `json:"account_id,omitempty"`
	ChannelID        int64  `json:"channel_id,omitempty"`
	Error            string `json:"error,omitempty"`
}
