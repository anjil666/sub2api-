package admin

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// flexFloat64 可以从 JSON number 或 string 反序列化为 float64
type flexFloat64 float64

func (f *flexFloat64) UnmarshalJSON(data []byte) error {
	// 尝试直接 number
	var n float64
	if err := json.Unmarshal(data, &n); err == nil {
		*f = flexFloat64(n)
		return nil
	}
	// 尝试 string → float64
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return fmt.Errorf("cannot parse %q as float64", s)
		}
		*f = flexFloat64(v)
		return nil
	}
	return fmt.Errorf("cannot unmarshal %s into float64", string(data))
}

// flexInt 可以从 JSON number 或 string 反序列化为 int
type flexInt int

func (f *flexInt) UnmarshalJSON(data []byte) error {
	var n int
	if err := json.Unmarshal(data, &n); err == nil {
		*f = flexInt(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		v, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("cannot parse %q as int", s)
		}
		*f = flexInt(v)
		return nil
	}
	return fmt.Errorf("cannot unmarshal %s into int", string(data))
}

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
	Name                string      `json:"name" binding:"required,max=200"`
	BaseURL             string      `json:"base_url" binding:"required,max=500"`
	CredentialMode      string      `json:"credential_mode" binding:"required,oneof=api_key login"`
	APIKey              string      `json:"api_key"`
	Email               string      `json:"email"`
	Password            string      `json:"password"`
	PriceMultiplier     flexFloat64 `json:"price_multiplier"`
	SyncEnabled         bool        `json:"sync_enabled"`
	SyncIntervalMinutes flexInt     `json:"sync_interval_minutes"`
}

type updateUpstreamSiteRequest struct {
	Name                string      `json:"name" binding:"required,max=200"`
	BaseURL             string      `json:"base_url" binding:"required,max=500"`
	CredentialMode      string      `json:"credential_mode" binding:"required,oneof=api_key login"`
	APIKey              string      `json:"api_key"`
	Email               string      `json:"email"`
	Password            string      `json:"password"`
	PriceMultiplier     flexFloat64 `json:"price_multiplier"`
	SyncEnabled         bool        `json:"sync_enabled"`
	SyncIntervalMinutes flexInt     `json:"sync_interval_minutes"`
	Status              string      `json:"status" binding:"omitempty,oneof=active disabled"`
}

type upstreamSiteResponse struct {
	ID                   int64   `json:"id"`
	Name                 string  `json:"name"`
	Platform             string  `json:"platform"`
	BaseURL              string  `json:"base_url"`
	CredentialMode       string  `json:"credential_mode"`
	APIKeyMasked         string  `json:"api_key_masked"`
	EmailMasked          string  `json:"email_masked"`
	HasPassword          bool    `json:"has_password"`
	PriceMultiplier      float64 `json:"price_multiplier"`
	SyncEnabled          bool    `json:"sync_enabled"`
	SyncIntervalMinutes  int     `json:"sync_interval_minutes"`
	LastSyncAt           *string `json:"last_sync_at"`
	LastSyncStatus       string  `json:"last_sync_status"`
	LastSyncError        string  `json:"last_sync_error"`
	LastSyncModelCount   int     `json:"last_sync_model_count"`
	Status               string  `json:"status"`
	ManagedResourceCount int     `json:"managed_resource_count"`
	CreatedAt            string  `json:"created_at"`
	UpdatedAt            string  `json:"updated_at"`
}

type managedResourceResponse struct {
	ID                     int64   `json:"id"`
	UpstreamKeyID          string  `json:"upstream_key_id"`
	UpstreamKeyPrefix      string  `json:"upstream_key_prefix"`
	UpstreamKeyName        string  `json:"upstream_key_name"`
	UpstreamGroupID        *int64  `json:"upstream_group_id"`
	ManagedGroupID         *int64  `json:"managed_group_id"`
	ManagedAccountID       *int64  `json:"managed_account_id"`
	ManagedChannelID       *int64  `json:"managed_channel_id"`
	PriceMultiplier        float64 `json:"price_multiplier"`
	UpstreamRateMultiplier float64 `json:"upstream_rate_multiplier"`
	ModelCount             int     `json:"model_count"`
	Status                 string  `json:"status"`
	LastSyncedAt           *string `json:"last_synced_at"`
	CreatedAt              string  `json:"created_at"`
	UpdatedAt              string  `json:"updated_at"`
}

func siteToResponse(s *service.UpstreamSite) *upstreamSiteResponse {
	resp := &upstreamSiteResponse{
		ID:                   s.ID,
		Name:                 s.Name,
		Platform:             s.Platform,
		BaseURL:              s.BaseURL,
		CredentialMode:       s.CredentialMode,
		APIKeyMasked:         maskAPIKey(s.APIKey),
		EmailMasked:          maskEmail(s.Email),
		HasPassword:          s.Password != "",
		PriceMultiplier:      s.PriceMultiplier,
		SyncEnabled:          s.SyncEnabled,
		SyncIntervalMinutes:  s.SyncIntervalMinutes,
		LastSyncStatus:       s.LastSyncStatus,
		LastSyncError:        s.LastSyncError,
		LastSyncModelCount:   s.LastSyncModelCount,
		Status:               s.Status,
		ManagedResourceCount: s.ManagedResourceCount,
		CreatedAt:            s.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:            s.UpdatedAt.Format("2006-01-02T15:04:05Z"),
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

func resourceToResponse(r *service.UpstreamManagedResource) *managedResourceResponse {
	resp := &managedResourceResponse{
		ID:                     r.ID,
		UpstreamKeyID:          r.UpstreamKeyID,
		UpstreamKeyPrefix:      r.UpstreamKeyPrefix,
		UpstreamKeyName:        r.UpstreamKeyName,
		UpstreamGroupID:        r.UpstreamGroupID,
		ManagedGroupID:         r.ManagedGroupID,
		ManagedAccountID:       r.ManagedAccountID,
		ManagedChannelID:       r.ManagedChannelID,
		PriceMultiplier:        r.PriceMultiplier,
		UpstreamRateMultiplier: r.UpstreamRateMultiplier,
		ModelCount:             r.ModelCount,
		Status:                 r.Status,
		CreatedAt:              r.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:              r.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if r.LastSyncedAt != nil {
		t := r.LastSyncedAt.Format("2006-01-02T15:04:05Z")
		resp.LastSyncedAt = &t
	}
	return resp
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

// maskEmail 脱敏邮箱
func maskEmail(email string) string {
	if email == "" {
		return ""
	}
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return "****"
	}
	local := parts[0]
	if len(local) <= 2 {
		return local + "****@" + parts[1]
	}
	return local[:2] + "****@" + parts[1]
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

	// 校验模式对应的必填字段
	if req.CredentialMode == "api_key" && req.APIKey == "" {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", "api_key is required for api_key mode"))
		return
	}
	if req.CredentialMode == "login" {
		if req.Email == "" || req.Password == "" {
			response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", "email and password are required for login mode"))
			return
		}
	}

	// 规范化 base_url
	req.BaseURL = strings.TrimRight(req.BaseURL, "/")

	// 默认值 — price_multiplier 现在是加价百分比，0 = 不加价
	if req.PriceMultiplier < 0 {
		req.PriceMultiplier = 0
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
		CredentialMode:      req.CredentialMode,
		APIKey:              req.APIKey,
		Email:               req.Email,
		Password:            req.Password,
		PriceMultiplier:     float64(req.PriceMultiplier),
		SyncEnabled:         req.SyncEnabled,
		SyncIntervalMinutes: int(req.SyncIntervalMinutes),
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

	if req.PriceMultiplier < 0 {
		req.PriceMultiplier = 0
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
		CredentialMode:      req.CredentialMode,
		APIKey:              req.APIKey, // 空字符串 = 不修改
		Email:               req.Email,
		Password:            req.Password,
		PriceMultiplier:     float64(req.PriceMultiplier),
		SyncEnabled:         req.SyncEnabled,
		SyncIntervalMinutes: int(req.SyncIntervalMinutes),
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

// Delete 删除上游站点（含级联清理本地分组/账号/渠道）
// DELETE /api/v1/admin/upstream-sites/:id
func (h *UpstreamHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_ID", "Invalid upstream site ID"))
		return
	}

	if err := h.syncService.DeleteSiteWithResources(c.Request.Context(), id); err != nil {
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
		log.Printf("[ERROR] POST /api/v1/admin/upstream-sites/%d/sync Error: %v", id, err)
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

// ListResources 列出站点托管资源
// GET /api/v1/admin/upstream-sites/:id/resources
func (h *UpstreamHandler) ListResources(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_ID", "Invalid upstream site ID"))
		return
	}

	resources, err := h.syncService.ListManagedResources(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]*managedResourceResponse, 0, len(resources))
	for _, r := range resources {
		out = append(out, resourceToResponse(r))
	}
	response.Success(c, out)
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

// UpdateResource 更新单个托管资源（倍率等）
// PUT /api/v1/admin/upstream-sites/:id/resources/:resourceId
func (h *UpstreamHandler) UpdateResource(c *gin.Context) {
	_, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_ID", "Invalid upstream site ID"))
		return
	}
	resourceID, err := strconv.ParseInt(c.Param("resourceId"), 10, 64)
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_ID", "Invalid resource ID"))
		return
	}

	var req struct {
		PriceMultiplier flexFloat64 `json:"price_multiplier"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}

	updated, err := h.syncService.UpdateResourceMultiplier(c.Request.Context(), resourceID, float64(req.PriceMultiplier))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, resourceToResponse(updated))
}

// ToggleResource 切换托管资源状态 (active ↔ disabled)
// POST /api/v1/admin/upstream-sites/:id/resources/:resourceId/toggle
func (h *UpstreamHandler) ToggleResource(c *gin.Context) {
	_, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_ID", "Invalid upstream site ID"))
		return
	}
	resourceID, err := strconv.ParseInt(c.Param("resourceId"), 10, 64)
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_ID", "Invalid resource ID"))
		return
	}

	updated, err := h.syncService.ToggleResource(c.Request.Context(), resourceID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, resourceToResponse(updated))
}
