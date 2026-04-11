package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// ReferralCommissionRepository 推荐返利记录仓库接口
type ReferralCommissionRepository interface {
	// Create 创建返利记录（INSERT ON CONFLICT DO NOTHING，幂等）
	Create(ctx context.Context, commission *ReferralCommission) (bool, error)
	// ListByReferrer 查询推荐人的返利记录
	ListByReferrer(ctx context.Context, referrerID int64, params pagination.PaginationParams) ([]ReferralCommission, *pagination.PaginationResult, error)
	// ListAll 查询所有返利记录（管理员）
	ListAll(ctx context.Context, params pagination.PaginationParams) ([]ReferralCommission, *pagination.PaginationResult, error)
	// GetStatsByReferrer 获取推荐人的返利统计
	GetStatsByReferrer(ctx context.Context, referrerID int64) (totalCommission float64, totalReferred int, err error)
	// GetReferredUsers 获取推荐人推荐的用户列表
	GetReferredUsers(ctx context.Context, referrerID int64, params pagination.PaginationParams) ([]ReferredUser, *pagination.PaginationResult, error)
}
