package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

var (
	ErrReferralDisabled = infraerrors.BadRequest("REFERRAL_DISABLED", "referral system is not enabled")
	ErrSelfReferral     = infraerrors.BadRequest("SELF_REFERRAL", "cannot refer yourself")
	ErrReferralNotFound = infraerrors.NotFound("REFERRAL_NOT_FOUND", "referral code not found")
)

// ReferralService 推荐返利服务
type ReferralService struct {
	userRepo       UserRepository
	commissionRepo ReferralCommissionRepository
	settingService *SettingService
	billingCache   BillingCache
	authCacheInv   APIKeyAuthCacheInvalidator
}

// NewReferralService 创建推荐返利服务实例
func NewReferralService(
	userRepo UserRepository,
	commissionRepo ReferralCommissionRepository,
	settingService *SettingService,
	billingCache BillingCache,
	authCacheInv APIKeyAuthCacheInvalidator,
) *ReferralService {
	return &ReferralService{
		userRepo:       userRepo,
		commissionRepo: commissionRepo,
		settingService: settingService,
		billingCache:   billingCache,
		authCacheInv:   authCacheInv,
	}
}

// GenerateReferralCode 生成推荐码（8位字母数字）
func GenerateReferralCode() (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	const length = 8
	code := make([]byte, length)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		code[i] = charset[n.Int64()]
	}
	return string(code), nil
}

// EnsureReferralCode 确保用户有推荐码，没有则生成
func (s *ReferralService) EnsureReferralCode(ctx context.Context, userID int64) (string, error) {
	code, _, err := s.userRepo.GetReferralFields(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("get referral fields: %w", err)
	}
	if code != nil && *code != "" {
		return *code, nil
	}
	// 生成新推荐码，最多重试3次避免冲突
	for i := 0; i < 3; i++ {
		newCode, err := GenerateReferralCode()
		if err != nil {
			return "", fmt.Errorf("generate referral code: %w", err)
		}
		if err := s.userRepo.UpdateReferralCode(ctx, userID, newCode); err != nil {
			continue // unique 约束冲突，重试
		}
		return newCode, nil
	}
	return "", fmt.Errorf("failed to generate unique referral code after retries")
}

// BindReferrer 注册时绑定推荐关系
func (s *ReferralService) BindReferrer(ctx context.Context, newUserID int64, referralCode string) error {
	if referralCode == "" {
		return nil
	}
	if !s.settingService.IsReferralEnabled(ctx) {
		return nil // 功能未启用，静默忽略
	}
	referrer, err := s.userRepo.GetByReferralCode(ctx, referralCode)
	if err != nil {
		return nil // 推荐码无效，静默忽略不影响注册
	}
	if referrer.ID == newUserID {
		return nil // 不能推荐自己
	}
	if err := s.userRepo.SetReferrerID(ctx, newUserID, referrer.ID); err != nil {
		log.Printf("[Referral] Failed to set referrer for user %d: %v", newUserID, err)
	}
	return nil
}

// CreditCommission 发放返利（由 sub2apipay 调用，幂等）
func (s *ReferralService) CreditCommission(ctx context.Context, referredID int64, orderCode string, orderAmount float64) (map[string]interface{}, error) {
	if !s.settingService.IsReferralEnabled(ctx) {
		return nil, ErrReferralDisabled
	}

	// 查找被推荐人的推荐人
	_, referrerIDPtr, err := s.userRepo.GetReferralFields(ctx, referredID)
	if err != nil {
		return nil, fmt.Errorf("get referral fields: %w", err)
	}
	if referrerIDPtr == nil {
		return map[string]interface{}{"credited": false, "reason": "no_referrer"}, nil
	}
	referrerID := *referrerIDPtr

	// 获取返利比例
	rate := s.settingService.GetReferralCommissionRate(ctx)
	if rate <= 0 || rate > 1 {
		return map[string]interface{}{"credited": false, "reason": "invalid_rate"}, nil
	}

	commissionAmount := orderAmount * rate

	// 创建返利记录（幂等：order_code UNIQUE，冲突忽略）
	commission := &ReferralCommission{
		ReferrerID:       referrerID,
		ReferredID:       referredID,
		OrderCode:        orderCode,
		OrderAmount:      orderAmount,
		CommissionRate:   rate,
		CommissionAmount: commissionAmount,
		Status:           "completed",
	}

	created, err := s.commissionRepo.Create(ctx, commission)
	if err != nil {
		return nil, fmt.Errorf("create commission: %w", err)
	}
	if !created {
		// 已经处理过（幂等）
		return map[string]interface{}{"credited": false, "reason": "duplicate_order"}, nil
	}

	// 增加推荐人余额
	if err := s.userRepo.UpdateBalance(ctx, referrerID, commissionAmount); err != nil {
		return nil, fmt.Errorf("update referrer balance: %w", err)
	}

	// 缓存失效
	if s.authCacheInv != nil {
		s.authCacheInv.InvalidateAuthCacheByUserID(ctx, referrerID)
	}
	if s.billingCache != nil {
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.billingCache.InvalidateUserBalance(cacheCtx, referrerID); err != nil {
				log.Printf("[Referral] invalidate balance cache failed: referrer_id=%d err=%v", referrerID, err)
			}
		}()
	}

	return map[string]interface{}{
		"credited":          true,
		"referrer_id":       referrerID,
		"commission_amount": commissionAmount,
		"commission_rate":   rate,
	}, nil
}

// GetStats 获取推荐统计
func (s *ReferralService) GetStats(ctx context.Context, userID int64) (*ReferralStats, error) {
	totalCommission, totalReferred, err := s.commissionRepo.GetStatsByReferrer(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get referral stats: %w", err)
	}

	rate := s.settingService.GetReferralCommissionRate(ctx)

	return &ReferralStats{
		TotalCommission: totalCommission,
		TotalReferred:   totalReferred,
		CommissionRate:  rate,
	}, nil
}

// GetReferredUsers 获取推荐用户列表（邮箱脱敏）
func (s *ReferralService) GetReferredUsers(ctx context.Context, userID int64, params pagination.PaginationParams) ([]ReferredUser, *pagination.PaginationResult, error) {
	users, paging, err := s.commissionRepo.GetReferredUsers(ctx, userID, params)
	if err != nil {
		return nil, nil, fmt.Errorf("get referred users: %w", err)
	}
	// 脱敏邮箱
	for i := range users {
		users[i].Email = MaskEmail(users[i].Email)
	}
	return users, paging, nil
}

// GetCommissions 获取推荐人的返利记录
func (s *ReferralService) GetCommissions(ctx context.Context, userID int64, params pagination.PaginationParams) ([]ReferralCommission, *pagination.PaginationResult, error) {
	return s.commissionRepo.ListByReferrer(ctx, userID, params)
}

// GetAllCommissions 获取所有返利记录（管理员）
func (s *ReferralService) GetAllCommissions(ctx context.Context, params pagination.PaginationParams) ([]ReferralCommission, *pagination.PaginationResult, error) {
	return s.commissionRepo.ListAll(ctx, params)
}
