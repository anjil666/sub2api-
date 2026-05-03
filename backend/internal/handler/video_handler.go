package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type VideoHandler struct {
	settingService      *service.SettingService
	billingCacheService *service.BillingCacheService
}

func NewVideoHandler(settingService *service.SettingService, billingCacheService *service.BillingCacheService) *VideoHandler {
	return &VideoHandler{
		settingService:      settingService,
		billingCacheService: billingCacheService,
	}
}

type videoGenerationRequest struct {
	Model       string `json:"model" binding:"required"`
	Prompt      string `json:"prompt" binding:"required"`
	AspectRatio string `json:"aspect_ratio"`
}

type videoModelInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (h *VideoHandler) getVideoSettings(c *gin.Context) (proxyURL, proxyToken string, err error) {
	settings, err := h.settingService.GetAllSettings(c.Request.Context())
	if err != nil {
		return "", "", err
	}
	if !settings.VideoStudioEnabled {
		return "", "", fmt.Errorf("video studio is disabled")
	}
	if settings.VideoProxyURL == "" {
		return "", "", fmt.Errorf("video proxy not configured")
	}
	return settings.VideoProxyURL, settings.VideoProxyToken, nil
}

// Generate handles POST /api/v1/user/video/generations
func (h *VideoHandler) Generate(c *gin.Context) {
	var req videoGenerationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	proxyURL, proxyToken, err := h.getVideoSettings(c)
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, err.Error())
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	settings, _ := h.settingService.GetAllSettings(c.Request.Context())
	price := settings.VideoDefaultPrice
	if price > 0 {
		uid, _ := userID.(int64)
		balance, err := h.billingCacheService.GetUserBalance(c.Request.Context(), uid)
		if err != nil || balance < price {
			response.Error(c, http.StatusPaymentRequired, "insufficient balance")
			return
		}
		h.billingCacheService.QueueDeductBalance(uid, price)
	}

	body, _ := json.Marshal(map[string]string{
		"model":        req.Model,
		"prompt":       req.Prompt,
		"aspect_ratio": req.AspectRatio,
	})

	proxyReq, _ := http.NewRequestWithContext(c.Request.Context(), "POST",
		proxyURL+"/v1/video/generations", bytes.NewReader(body))
	proxyReq.Header.Set("Content-Type", "application/json")
	if proxyToken != "" {
		proxyReq.Header.Set("Authorization", "Bearer "+proxyToken)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		response.Error(c, http.StatusBadGateway, "proxy error: "+err.Error())
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, "application/json", respBody)
}

// GetStatus handles GET /api/v1/user/video/generations/:id
func (h *VideoHandler) GetStatus(c *gin.Context) {
	taskID := c.Param("id")

	proxyURL, proxyToken, err := h.getVideoSettings(c)
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, err.Error())
		return
	}

	proxyReq, _ := http.NewRequestWithContext(c.Request.Context(), "GET",
		proxyURL+"/v1/video/generations/"+taskID, nil)
	if proxyToken != "" {
		proxyReq.Header.Set("Authorization", "Bearer "+proxyToken)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		response.Error(c, http.StatusBadGateway, "proxy error: "+err.Error())
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, "application/json", respBody)
}

// GetModels handles GET /api/v1/user/video/models
func (h *VideoHandler) GetModels(c *gin.Context) {
	settings, err := h.settingService.GetAllSettings(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to load settings")
		return
	}

	if !settings.VideoStudioEnabled {
		c.JSON(http.StatusOK, gin.H{"models": []videoModelInfo{}, "price": 0})
		return
	}

	var models []videoModelInfo
	if settings.VideoModelVeo31Enabled {
		models = append(models, videoModelInfo{ID: "veo-3.1", Name: "VEO 3.1"})
	}
	if settings.VideoModelVeo20Enabled {
		models = append(models, videoModelInfo{ID: "veo-2.0", Name: "VEO 2.0"})
	}
	if settings.VideoModelSeedance {
		models = append(models, videoModelInfo{ID: "seedance-2.0", Name: "Seedance 2.0"})
	}
	if settings.VideoModelGrok {
		models = append(models, videoModelInfo{ID: "grok-video", Name: "Grok Video"})
	}

	c.JSON(http.StatusOK, gin.H{
		"models": models,
		"price":  settings.VideoDefaultPrice,
	})
}
