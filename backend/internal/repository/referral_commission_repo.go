package repository

import (
	"context"
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type referralCommissionRepository struct {
	sql sqlExecutor
}

// NewReferralCommissionRepository 创建推荐返利记录仓库
func NewReferralCommissionRepository(sqlDB *sql.DB) service.ReferralCommissionRepository {
	return &referralCommissionRepository{sql: sqlDB}
}

// Create 创建返利记录（幂等：INSERT ON CONFLICT DO NOTHING）
func (r *referralCommissionRepository) Create(ctx context.Context, c *service.ReferralCommission) (bool, error) {
	query := `
		INSERT INTO referral_commissions (referrer_id, referred_id, order_code, order_amount, commission_rate, commission_amount, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (order_code) DO NOTHING
		RETURNING id`
	rows, err := r.sql.QueryContext(ctx, query,
		c.ReferrerID, c.ReferredID, c.OrderCode, c.OrderAmount, c.CommissionRate, c.CommissionAmount, c.Status)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if rows.Next() {
		if err := rows.Scan(&c.ID); err != nil {
			return false, err
		}
		return true, nil // 成功创建
	}
	return false, nil // 冲突，已存在
}

// ListByReferrer 查询推荐人的返利记录
func (r *referralCommissionRepository) ListByReferrer(ctx context.Context, referrerID int64, params pagination.PaginationParams) ([]service.ReferralCommission, *pagination.PaginationResult, error) {
	// 计数
	countQuery := `SELECT COUNT(*) FROM referral_commissions WHERE referrer_id = $1`
	countRows, err := r.sql.QueryContext(ctx, countQuery, referrerID)
	if err != nil {
		return nil, nil, err
	}
	defer countRows.Close()
	var total int
	if countRows.Next() {
		if err := countRows.Scan(&total); err != nil {
			return nil, nil, err
		}
	}

	page, pageSize := normalizePagination(params)
	offset := (page - 1) * pageSize

	query := `
		SELECT rc.id, rc.referrer_id, rc.referred_id, rc.order_code, rc.order_amount,
		       rc.commission_rate, rc.commission_amount, rc.status, rc.created_at, rc.updated_at,
		       COALESCE(u.email, '') as referred_email
		FROM referral_commissions rc
		LEFT JOIN users u ON u.id = rc.referred_id
		WHERE rc.referrer_id = $1
		ORDER BY rc.created_at DESC
		LIMIT $2 OFFSET $3`
	rows, err := r.sql.QueryContext(ctx, query, referrerID, pageSize, offset)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var results []service.ReferralCommission
	for rows.Next() {
		var c service.ReferralCommission
		if err := rows.Scan(&c.ID, &c.ReferrerID, &c.ReferredID, &c.OrderCode, &c.OrderAmount,
			&c.CommissionRate, &c.CommissionAmount, &c.Status, &c.CreatedAt, &c.UpdatedAt,
			&c.ReferredEmail); err != nil {
			return nil, nil, err
		}
		results = append(results, c)
	}

	paging := &pagination.PaginationResult{
		Total:       total,
		Page:        page,
		PageSize:    pageSize,
		TotalPages:  (total + pageSize - 1) / pageSize,
		HasNextPage: page*pageSize < total,
	}
	return results, paging, nil
}

// ListAll 查询所有返利记录（管理员）
func (r *referralCommissionRepository) ListAll(ctx context.Context, params pagination.PaginationParams) ([]service.ReferralCommission, *pagination.PaginationResult, error) {
	countQuery := `SELECT COUNT(*) FROM referral_commissions`
	countRows, err := r.sql.QueryContext(ctx, countQuery)
	if err != nil {
		return nil, nil, err
	}
	defer countRows.Close()
	var total int
	if countRows.Next() {
		if err := countRows.Scan(&total); err != nil {
			return nil, nil, err
		}
	}

	page, pageSize := normalizePagination(params)
	offset := (page - 1) * pageSize

	query := `
		SELECT rc.id, rc.referrer_id, rc.referred_id, rc.order_code, rc.order_amount,
		       rc.commission_rate, rc.commission_amount, rc.status, rc.created_at, rc.updated_at,
		       COALESCE(u.email, '') as referred_email
		FROM referral_commissions rc
		LEFT JOIN users u ON u.id = rc.referred_id
		ORDER BY rc.created_at DESC
		LIMIT $1 OFFSET $2`
	rows, err := r.sql.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var results []service.ReferralCommission
	for rows.Next() {
		var c service.ReferralCommission
		if err := rows.Scan(&c.ID, &c.ReferrerID, &c.ReferredID, &c.OrderCode, &c.OrderAmount,
			&c.CommissionRate, &c.CommissionAmount, &c.Status, &c.CreatedAt, &c.UpdatedAt,
			&c.ReferredEmail); err != nil {
			return nil, nil, err
		}
		results = append(results, c)
	}

	paging := &pagination.PaginationResult{
		Total:       total,
		Page:        page,
		PageSize:    pageSize,
		TotalPages:  (total + pageSize - 1) / pageSize,
		HasNextPage: page*pageSize < total,
	}
	return results, paging, nil
}

// GetStatsByReferrer 获取推荐人统计
func (r *referralCommissionRepository) GetStatsByReferrer(ctx context.Context, referrerID int64) (totalCommission float64, totalReferred int, err error) {
	query := `
		SELECT COALESCE(SUM(commission_amount), 0) as total_commission,
		       COUNT(DISTINCT referred_id) as total_referred
		FROM referral_commissions
		WHERE referrer_id = $1`
	rows, err := r.sql.QueryContext(ctx, query, referrerID)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	if rows.Next() {
		if err := rows.Scan(&totalCommission, &totalReferred); err != nil {
			return 0, 0, err
		}
	}
	return totalCommission, totalReferred, nil
}

// GetReferredUsers 获取推荐用户列表
func (r *referralCommissionRepository) GetReferredUsers(ctx context.Context, referrerID int64, params pagination.PaginationParams) ([]service.ReferredUser, *pagination.PaginationResult, error) {
	countQuery := `SELECT COUNT(*) FROM users WHERE referrer_id = $1 AND deleted_at IS NULL`
	countRows, err := r.sql.QueryContext(ctx, countQuery, referrerID)
	if err != nil {
		return nil, nil, err
	}
	defer countRows.Close()
	var total int
	if countRows.Next() {
		if err := countRows.Scan(&total); err != nil {
			return nil, nil, err
		}
	}

	page, pageSize := normalizePagination(params)
	offset := (page - 1) * pageSize

	query := `
		SELECT id, email, created_at
		FROM users
		WHERE referrer_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	rows, err := r.sql.QueryContext(ctx, query, referrerID, pageSize, offset)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var results []service.ReferredUser
	for rows.Next() {
		var u service.ReferredUser
		if err := rows.Scan(&u.ID, &u.Email, &u.CreatedAt); err != nil {
			return nil, nil, err
		}
		results = append(results, u)
	}

	paging := &pagination.PaginationResult{
		Total:       total,
		Page:        page,
		PageSize:    pageSize,
		TotalPages:  (total + pageSize - 1) / pageSize,
		HasNextPage: page*pageSize < total,
	}
	return results, paging, nil
}

func normalizePagination(params pagination.PaginationParams) (page, pageSize int) {
	page = params.Page
	pageSize = params.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
