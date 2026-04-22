package handler

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// HealthStatusHandler provides user-facing health status endpoints.
type HealthStatusHandler struct {
	healthProbeSvc *service.HealthProbeService
}

// NewHealthStatusHandler creates a new HealthStatusHandler.
func NewHealthStatusHandler(healthProbeSvc *service.HealthProbeService) *HealthStatusHandler {
	return &HealthStatusHandler{healthProbeSvc: healthProbeSvc}
}

// GetConfig GET /health-status/config — exposes non-sensitive probe config to users
func (h *HealthStatusHandler) GetConfig(c *gin.Context) {
	ctx := c.Request.Context()
	cfg, err := h.healthProbeSvc.GetConfig(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"timeout_seconds": cfg.TimeoutSeconds,
	})
}

// GetLatest GET /health-status/latest
func (h *HealthStatusHandler) GetLatest(c *gin.Context) {
	ctx := c.Request.Context()
	results, err := h.healthProbeSvc.GetLatestResultsForUsers(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Enrich with group metadata
	h.healthProbeSvc.EnrichResultsWithGroupInfo(ctx, results)

	// Return a simplified view for users (no account IDs, no error details)
	type userResult struct {
		GroupID        int64   `json:"group_id"`
		GroupName      string  `json:"group_name"`
		RateMultiplier float64 `json:"rate_multiplier"`
		Platform       string  `json:"platform"`
		ProbeModel     string  `json:"probe_model"`
		Status         int     `json:"status"`
		LatencyMs      int     `json:"latency_ms"`
		CheckedAt      string  `json:"checked_at"`
	}

	var userResults []userResult
	for _, r := range results {
		userResults = append(userResults, userResult{
			GroupID:        r.GroupID,
			GroupName:      r.GroupName,
			RateMultiplier: r.RateMultiplier,
			Platform:       r.Platform,
			ProbeModel:     r.ProbeModel,
			Status:         r.Status,
			LatencyMs:      r.LatencyMs,
			CheckedAt:      r.CheckedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	c.JSON(http.StatusOK, userResults)
}

// GetGroupSummaries GET /health-status/groups/:id/summaries
func (h *HealthStatusHandler) GetGroupSummaries(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}

	hours := 24
	if hStr := c.Query("hours"); hStr != "" {
		if v, err := strconv.Atoi(hStr); err == nil && v > 0 && v <= 168 {
			hours = v
		}
	}

	summaries, err := h.healthProbeSvc.GetGroupSummaries(c.Request.Context(), groupID, hours)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summaries)
}

// GetAllSummaries GET /health-status/summaries
func (h *HealthStatusHandler) GetAllSummaries(c *gin.Context) {
	hours := 24
	if hStr := c.Query("hours"); hStr != "" {
		if v, err := strconv.Atoi(hStr); err == nil && v > 0 && v <= 168 {
			hours = v
		}
	}

	summaries, err := h.healthProbeSvc.GetAllSummaries(c.Request.Context(), hours)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summaries)
}
