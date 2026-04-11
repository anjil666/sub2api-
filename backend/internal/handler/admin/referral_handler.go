package admin

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ReferralHandler 推荐返利管理端处理器
type ReferralHandler struct {
	referralService *service.ReferralService
}

// NewReferralHandler 创建推荐返利管理端处理器
func NewReferralHandler(referralService *service.ReferralService) *ReferralHandler {
	return &ReferralHandler{referralService: referralService}
}

// GetCommissions 获取所有返利记录
// GET /api/v1/admin/referral/commissions
func (h *ReferralHandler) GetCommissions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	commissions, paging, err := h.referralService.GetAllCommissions(c.Request.Context(), pagination.PaginationParams{
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
			"referrer_id":       comm.ReferrerID,
			"referred_id":       comm.ReferredID,
			"referred_email":    comm.ReferredEmail,
			"order_code":        comm.OrderCode,
			"order_amount":      comm.OrderAmount,
			"commission_rate":   comm.CommissionRate,
			"commission_amount": comm.CommissionAmount,
			"status":            comm.Status,
			"created_at":        comm.CreatedAt,
		})
	}
	if items == nil {
		items = []gin.H{}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"items":      items,
			"total":      paging.Total,
			"page":       paging.Page,
			"page_size":  paging.PageSize,
			"pages":      paging.TotalPages,
		},
	})
}

// CreditCommission 手动触发返利（由 sub2apipay 调用）
// POST /api/v1/admin/referral/credit-commission
func (h *ReferralHandler) CreditCommission(c *gin.Context) {
	var req struct {
		UserID      int64   `json:"user_id" binding:"required"`
		OrderCode   string  `json:"order_code" binding:"required"`
		OrderAmount float64 `json:"order_amount" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	result, err := h.referralService.CreditCommission(c.Request.Context(), req.UserID, req.OrderCode, req.OrderAmount)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, result)
}
