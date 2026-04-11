package admin

import (
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// UpstreamHandler handles admin upstream site management
type UpstreamHandler struct {
	siteRepo    service.UpstreamSiteRepository
	syncService *service.UpstreamSyncService
}

// NewUpstreamHandler creates a new admin upstream handler
func NewUpstreamHandler(siteRepo service.UpstreamSiteRepository, syncService *service.UpstreamSyncService) *UpstreamHandler {
	return &UpstreamHandler{siteRepo: siteRepo, syncService: syncService}
}

// --- Request / Response types ---

type createUpstreamSiteRequest struct {
	Name                string  `json:"name" binding:"required,max=200"`
	BaseURL             string  `json:"base_url" binding:"required,max=500"`
	APIKey              string  `json:"api_key" binding:"required"`
	PriceMultiplier     float64 `json:"price_multiplier"`
	SyncEnabled         bool    `json:"sync_enabled"`
	SyncIntervalMinutes int     `json:"sync_interval_minutes"`
}

type updateUpstreamSiteRequest struct {
	Name                string  `json:"name" binding:"required,max=200"`
	BaseURL             string  `json:"base_url" binding:"required,max=500"`
	APIKey              string  `json:"api_key"`
	PriceMultiplier     float64 `json:"price_multiplier"`
	SyncEnabled         bool    `json:"sync_enabled"`
	SyncIntervalMinutes int     `json:"sync_interval_minutes"`
	Status              string  `json:"status" binding:"omitempty,oneof=active disabled"`
}

type upstreamSiteResponse struct {
	ID                  int64   `json:"id"`
	Name                string  `json:"name"`
	Platform            string  `json:"platform"`
	BaseURL             string  `json:"base_url"`
	APIKeyMasked        string  `json:"api_key_masked"`
	PriceMultiplier     float64 `json:"price_multiplier"`
	SyncEnabled         bool    `json:"sync_enabled"`
	SyncIntervalMinutes int     `json:"sync_interval_minutes"`
	LastSyncAt          *string `json:"last_sync_at"`
	LastSyncStatus      string  `json:"last_sync_status"`
	LastSyncError       string  `json:"last_sync_error"`
	LastSyncModelCount  int     `json:"last_sync_model_count"`
	Status              string  `json:"status"`
	ManagedGroupID      *int64  `json:"managed_group_id"`
	ManagedAccountID    *int64  `json:"managed_account_id"`
	ManagedChannelID    *int64  `json:"managed_channel_id"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
}

func siteToResponse(s *service.UpstreamSite) *upstreamSiteResponse {
	resp := &upstreamSiteResponse{
		ID:                  s.ID,
		Name:                s.Name,
		Platform:            s.Platform,
		BaseURL:             s.BaseURL,
		APIKeyMasked:        maskAPIKey(s.APIKey),
		PriceMultiplier:     s.PriceMultiplier,
		SyncEnabled:         s.SyncEnabled,
		SyncIntervalMinutes: s.SyncIntervalMinutes,
		LastSyncStatus:      s.LastSyncStatus,
		LastSyncError:       s.LastSyncError,
		LastSyncModelCount:  s.LastSyncModelCount,
		Status:              s.Status,
		ManagedGroupID:      s.ManagedGroupID,
		ManagedAccountID:    s.ManagedAccountID,
		ManagedChannelID:    s.ManagedChannelID,
		CreatedAt:           s.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:           s.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if s.LastSyncAt != nil {
		t := s.LastSyncAt.Format("2006-01-02T15:04:05Z")
		resp.LastSyncAt = &t
	}
	return resp
}

func siteListItemToResponse(s *service.UpstreamSite) *upstreamSiteResponse {
	return siteToResponse(s)
}

// maskAPIKey 脱敏 API Key
func maskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "sk-****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

// --- Handler methods ---

// List 获取上游站点列表
// GET /api/v1/admin/upstream-sites
func (h *UpstreamHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	status := c.Query("status")
	search := strings.TrimSpace(c.Query("search"))
	if len(search) > 100 {
		search = search[:100]
	}

	sites, pag, err := h.siteRepo.List(c.Request.Context(), pagination.PaginationParams{Page: page, PageSize: pageSize}, status, search)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]*upstreamSiteResponse, 0, len(sites))
	for i := range sites {
		out = append(out, siteListItemToResponse(&sites[i]))
	}
	response.Paginated(c, out, pag.Total, page, pageSize)
}

// GetByID 获取上游站点详情
// GET /api/v1/admin/upstream-sites/:id
func (h *UpstreamHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_ID", "Invalid upstream site ID"))
		return
	}

	site, err := h.siteRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, siteToResponse(site))
}

// Create 新建上游站点
// POST /api/v1/admin/upstream-sites
func (h *UpstreamHandler) Create(c *gin.Context) {
	var req createUpstreamSiteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}

	// 规范化 base_url
	req.BaseURL = strings.TrimRight(req.BaseURL, "/")

	// 默认值
	if req.PriceMultiplier <= 0 {
		req.PriceMultiplier = 1.0
	}
	if req.SyncIntervalMinutes <= 0 {
		req.SyncIntervalMinutes = 60
	}

	// 检查 base_url 唯一
	exists, err := h.siteRepo.ExistsByBaseURL(c.Request.Context(), req.BaseURL)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if exists {
		response.ErrorFrom(c, service.ErrUpstreamSiteExists)
		return
	}

	site := &service.UpstreamSite{
		Name:                req.Name,
		Platform:            "sub2api",
		BaseURL:             req.BaseURL,
		APIKey:              req.APIKey,
		PriceMultiplier:     req.PriceMultiplier,
		SyncEnabled:         req.SyncEnabled,
		SyncIntervalMinutes: req.SyncIntervalMinutes,
		Status:              service.StatusActive,
	}

	if err := h.siteRepo.Create(c.Request.Context(), site); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, siteToResponse(site))
}

// Update 更新上游站点
// PUT /api/v1/admin/upstream-sites/:id
func (h *UpstreamHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_ID", "Invalid upstream site ID"))
		return
	}

	var req updateUpstreamSiteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}

	req.BaseURL = strings.TrimRight(req.BaseURL, "/")

	// 检查 base_url 唯一（排除自身）
	exists, err := h.siteRepo.ExistsByBaseURLExcluding(c.Request.Context(), req.BaseURL, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if exists {
		response.ErrorFrom(c, service.ErrUpstreamSiteExists)
		return
	}

	if req.PriceMultiplier <= 0 {
		req.PriceMultiplier = 1.0
	}
	if req.SyncIntervalMinutes <= 0 {
		req.SyncIntervalMinutes = 60
	}
	if req.Status == "" {
		req.Status = service.StatusActive
	}

	site := &service.UpstreamSite{
		ID:                  id,
		Name:                req.Name,
		BaseURL:             req.BaseURL,
		APIKey:              req.APIKey, // 空字符串 = 不修改
		PriceMultiplier:     req.PriceMultiplier,
		SyncEnabled:         req.SyncEnabled,
		SyncIntervalMinutes: req.SyncIntervalMinutes,
		Status:              req.Status,
	}

	if err := h.siteRepo.Update(c.Request.Context(), site); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 重新获取完整数据
	updated, err := h.siteRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, siteToResponse(updated))
}

// Delete 删除上游站点
// DELETE /api/v1/admin/upstream-sites/:id
func (h *UpstreamHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_ID", "Invalid upstream site ID"))
		return
	}

	if err := h.siteRepo.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nil)
}

// Sync 立即同步指定站点
// POST /api/v1/admin/upstream-sites/:id/sync
func (h *UpstreamHandler) Sync(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_ID", "Invalid upstream site ID"))
		return
	}

	result, err := h.syncService.SyncSiteNow(c.Request.Context(), id)
	if err != nil {
		// 同步失败但仍返回 result
		if result != nil {
			response.Success(c, result)
			return
		}
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, result)
}

// GetBalance 查询上游余额
// GET /api/v1/admin/upstream-sites/:id/balance
func (h *UpstreamHandler) GetBalance(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_ID", "Invalid upstream site ID"))
		return
	}

	balance, err := h.syncService.CheckUpstreamBalance(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, balance)
}

// GetModels 预览上游模型列表
// GET /api/v1/admin/upstream-sites/:id/models
func (h *UpstreamHandler) GetModels(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_ID", "Invalid upstream site ID"))
		return
	}

	models, err := h.syncService.FetchUpstreamModels(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, models)
}

// Toggle 切换站点状态 (active ↔ disabled)
// POST /api/v1/admin/upstream-sites/:id/toggle
func (h *UpstreamHandler) Toggle(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_ID", "Invalid upstream site ID"))
		return
	}

	site, err := h.siteRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if site.Status == service.StatusActive {
		site.Status = service.StatusDisabled
	} else {
		site.Status = service.StatusActive
	}

	if err := h.siteRepo.Update(c.Request.Context(), site); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, siteToResponse(site))
}
