package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ReferralHandler 推荐返利用户端处理器
type ReferralHandler struct {
	referralService *service.ReferralService
	settingService  *service.SettingService
}

// NewReferralHandler 创建推荐返利处理器
func NewReferralHandler(referralService *service.ReferralService, settingService *service.SettingService) *ReferralHandler {
	return &ReferralHandler{
		referralService: referralService,
		settingService:  settingService,
	}
}

// GetReferralInfo 获取用户推荐信息（推荐码 + 统计）
// GET /api/v1/user/referral/info
func (h *ReferralHandler) GetReferralInfo(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	code, err := h.referralService.EnsureReferralCode(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	stats, err := h.referralService.GetStats(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	frontendURL := h.settingService.GetFrontendURL(c.Request.Context())

	response.Success(c, gin.H{
		"referral_code":    code,
		"referral_link":    frontendURL + "/register?ref=" + code,
		"total_commission": stats.TotalCommission,
		"total_referred":   stats.TotalReferred,
		"commission_rate":  stats.CommissionRate,
	})
}

// GetReferredUsers 获取推荐用户列表
// GET /api/v1/user/referral/users
func (h *ReferralHandler) GetReferredUsers(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	users, paging, err := h.referralService.GetReferredUsers(c.Request.Context(), subject.UserID, pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if users == nil {
		users = []service.ReferredUser{}
	}

	response.Success(c, response.PaginatedData{
		Items:    users,
		Total:    paging.Total,
		Page:     paging.Page,
		PageSize: paging.PageSize,
		Pages:    paging.Pages,
	})
}

// GetCommissions 获取返利记录
// GET /api/v1/user/referral/commissions
func (h *ReferralHandler) GetCommissions(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	commissions, paging, err := h.referralService.GetCommissions(c.Request.Context(), subject.UserID, pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	var items []gin.H
	for _, comm := range commissions {
		items = append(items, gin.H{
			"id":                comm.ID,
			"order_code":        comm.OrderCode,
			"order_amount":      comm.OrderAmount,
			"commission_rate":   comm.CommissionRate,
			"commission_amount": comm.CommissionAmount,
			"status":            comm.Status,
			"referred_email":    service.MaskEmail(comm.ReferredEmail),
			"created_at":        comm.CreatedAt,
		})
	}
	if items == nil {
		items = []gin.H{}
	}

	response.Success(c, response.PaginatedData{
		Items:    items,
		Total:    paging.Total,
		Page:     paging.Page,
		PageSize: paging.PageSize,
		Pages:    paging.Pages,
	})
}
