package service

import "time"

// ReferralCommission 推荐返利记录
type ReferralCommission struct {
	ID               int64
	ReferrerID       int64
	ReferredID       int64
	OrderCode        string
	OrderAmount      float64
	CommissionRate   float64
	CommissionAmount float64
	Status           string
	CreatedAt        time.Time
	UpdatedAt        time.Time

	// 视图字段（JOIN 查询用）
	ReferredEmail string
}

// ReferralStats 推荐统计
type ReferralStats struct {
	TotalCommission float64 `json:"total_commission"`
	TotalReferred   int     `json:"total_referred"`
	CommissionRate  float64 `json:"commission_rate"`
}

// ReferredUser 被推荐用户（脱敏）
type ReferredUser struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"` // 脱敏后的邮箱
	CreatedAt time.Time `json:"created_at"`
}
