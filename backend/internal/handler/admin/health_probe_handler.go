package admin

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// HealthProbeHandler handles admin health probe management.
type HealthProbeHandler struct {
	healthProbeSvc *service.HealthProbeService
}

// NewHealthProbeHandler creates a new HealthProbeHandler.
func NewHealthProbeHandler(healthProbeSvc *service.HealthProbeService) *HealthProbeHandler {
	return &HealthProbeHandler{healthProbeSvc: healthProbeSvc}
}

// GetConfig GET /admin/health-probe/config
func (h *HealthProbeHandler) GetConfig(c *gin.Context) {
	cfg, err := h.healthProbeSvc.GetConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, cfg)
}

type updateHealthProbeConfigRequest struct {
	Enabled                *bool   `json:"enabled"`
	IntervalMinutes        *int    `json:"interval_minutes"`
	TimeoutSeconds         *int    `json:"timeout_seconds"`
	RetentionHours         *int    `json:"retention_hours"`
	SlowThresholdMs        *int    `json:"slow_threshold_ms"`
	WebhookEnabled         *bool   `json:"webhook_enabled"`
	WebhookURL             *string `json:"webhook_url"`
	WebhookDebounceCount   *int    `json:"webhook_debounce_count"`
	WebhookCooldownMinutes *int    `json:"webhook_cooldown_minutes"`
}

// UpdateConfig PUT /admin/health-probe/config
func (h *HealthProbeHandler) UpdateConfig(c *gin.Context) {
	var req updateHealthProbeConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	ctx := c.Request.Context()
	cfg, err := h.healthProbeSvc.GetConfig(ctx)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	// Apply partial updates
	if req.Enabled != nil {
		cfg.Enabled = *req.Enabled
	}
	if req.IntervalMinutes != nil {
		cfg.IntervalMinutes = *req.IntervalMinutes
	}
	if req.TimeoutSeconds != nil {
		cfg.TimeoutSeconds = *req.TimeoutSeconds
	}
	if req.RetentionHours != nil {
		cfg.RetentionHours = *req.RetentionHours
	}
	if req.SlowThresholdMs != nil {
		cfg.SlowThresholdMs = *req.SlowThresholdMs
	}
	if req.WebhookEnabled != nil {
		cfg.WebhookEnabled = *req.WebhookEnabled
	}
	if req.WebhookURL != nil {
		cfg.WebhookURL = *req.WebhookURL
	}
	if req.WebhookDebounceCount != nil {
		cfg.WebhookDebounceCount = *req.WebhookDebounceCount
	}
	if req.WebhookCooldownMinutes != nil {
		cfg.WebhookCooldownMinutes = *req.WebhookCooldownMinutes
	}

	if err := h.healthProbeSvc.UpdateConfig(ctx, cfg); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, cfg)
}

// TriggerProbe POST /admin/health-probe/trigger
func (h *HealthProbeHandler) TriggerProbe(c *gin.Context) {
	h.healthProbeSvc.RunManualProbe()
	c.JSON(http.StatusOK, gin.H{"message": "probe triggered"})
}

// GetLatestResults GET /admin/health-probe/latest
func (h *HealthProbeHandler) GetLatestResults(c *gin.Context) {
	results, err := h.healthProbeSvc.GetLatestResults(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, results)
}

// GetGroupResults GET /admin/health-probe/groups/:id/results
func (h *HealthProbeHandler) GetGroupResults(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid group id")
		return
	}

	hours := 24
	if h := c.Query("hours"); h != "" {
		if v, err := strconv.Atoi(h); err == nil && v > 0 {
			hours = v
		}
	}

	limit := 100
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	results, err := h.healthProbeSvc.GetGroupResults(c.Request.Context(), groupID, hours, limit)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, results)
}

// GetGroupSummaries GET /admin/health-probe/groups/:id/summaries
func (h *HealthProbeHandler) GetGroupSummaries(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid group id")
		return
	}

	hours := 24
	if h := c.Query("hours"); h != "" {
		if v, err := strconv.Atoi(h); err == nil && v > 0 {
			hours = v
		}
	}

	summaries, err := h.healthProbeSvc.GetGroupSummaries(c.Request.Context(), groupID, hours)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, summaries)
}

// GetAllSummaries GET /admin/health-probe/summaries
func (h *HealthProbeHandler) GetAllSummaries(c *gin.Context) {
	hours := 24
	if h := c.Query("hours"); h != "" {
		if v, err := strconv.Atoi(h); err == nil && v > 0 {
			hours = v
		}
	}

	summaries, err := h.healthProbeSvc.GetAllSummaries(c.Request.Context(), hours)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, summaries)
}
