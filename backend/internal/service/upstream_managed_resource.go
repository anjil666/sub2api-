package service

import (
	"strings"
	"time"
)

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
	ModelFilter           string // comma-separated patterns (e.g. "gpt-image-*,dall-e-*"); empty = no filter
	Status                string // "active", "disabled"
	DisabledBy            string // "": not disabled, "auto": upstream removed, "manual": admin toggled
	LastSyncedAt          *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time

	// 非持久化字段（仅同步期间使用）
	UpstreamGroupDescription string `json:"-"` // 上游分组描述
}

// FilterModels applies the ModelFilter patterns to a list of models.
// Returns only models matching at least one pattern.
// If ModelFilter is empty, returns all models unchanged.
func (r *UpstreamManagedResource) FilterModels(models []UpstreamModelInfo) []UpstreamModelInfo {
	if r.ModelFilter == "" {
		return models
	}
	patterns := r.parseModelFilter()
	if len(patterns) == 0 {
		return models
	}
	var filtered []UpstreamModelInfo
	for _, m := range models {
		if matchesAnyPattern(m.ID, patterns) {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

func (r *UpstreamManagedResource) parseModelFilter() []string {
	raw := strings.TrimSpace(r.ModelFilter)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var patterns []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			patterns = append(patterns, strings.ToLower(p))
		}
	}
	return patterns
}

// matchesAnyPattern checks if modelID matches any of the given patterns.
// Patterns support "*" suffix for prefix matching, e.g. "gpt-image-*".
func matchesAnyPattern(modelID string, patterns []string) bool {
	lower := strings.ToLower(modelID)
	for _, p := range patterns {
		if strings.HasSuffix(p, "*") {
			prefix := strings.TrimSuffix(p, "*")
			if strings.HasPrefix(lower, prefix) {
				return true
			}
		} else {
			if lower == p {
				return true
			}
		}
	}
	return false
}
