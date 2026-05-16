package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type VideoHandler struct {
	settingService      *service.SettingService
	billingCacheService *service.BillingCacheService
	usageService        *service.UsageService
}

func NewVideoHandler(settingService *service.SettingService, billingCacheService *service.BillingCacheService, usageService *service.UsageService) *VideoHandler {
	return &VideoHandler{
		settingService:      settingService,
		billingCacheService: billingCacheService,
		usageService:        usageService,
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

	authSubject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	settings, _ := h.settingService.GetAllSettings(c.Request.Context())
	price := settings.VideoDefaultPrice
	if price > 0 {
		balance, err := h.billingCacheService.GetUserBalance(c.Request.Context(), authSubject.UserID)
		if err != nil || balance < price {
			response.Error(c, http.StatusPaymentRequired, "insufficient balance")
			return
		}
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

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		response.Error(c, http.StatusBadGateway, "proxy error: "+err.Error())
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 && price > 0 {
		go func() {
			_, _ = h.usageService.Create(context.Background(), service.CreateUsageLogRequest{
				UserID:         authSubject.UserID,
				RequestID:      uuid.New().String(),
				Model:          req.Model,
				TotalCost:      price,
				ActualCost:     price,
				RateMultiplier: 1.0,
			})
		}()
	}

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

	client := &http.Client{Timeout: 300 * time.Second}
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

// EnhancePrompt handles POST /api/v1/user/video/prompt/enhance
func (h *VideoHandler) EnhancePrompt(c *gin.Context) {
	var req struct {
		Prompt string `json:"prompt" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	proxyURL, proxyToken, err := h.getVideoSettings(c)
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, err.Error())
		return
	}

	body, _ := json.Marshal(map[string]string{"prompt": req.Prompt})
	proxyReq, _ := http.NewRequestWithContext(c.Request.Context(), "POST",
		proxyURL+"/v1/prompt/enhance", bytes.NewReader(body))
	proxyReq.Header.Set("Content-Type", "application/json")
	if proxyToken != "" {
		proxyReq.Header.Set("Authorization", "Bearer "+proxyToken)
	}

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		response.Error(c, http.StatusBadGateway, "proxy error: "+err.Error())
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, "application/json", respBody)
}

// GenerateFromImage handles POST /api/v1/user/video/img2video
func (h *VideoHandler) GenerateFromImage(c *gin.Context) {
	proxyURL, proxyToken, err := h.getVideoSettings(c)
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, err.Error())
		return
	}

	authSubject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	settings, _ := h.settingService.GetAllSettings(c.Request.Context())
	price := settings.VideoDefaultPrice
	if price > 0 {
		balance, err := h.billingCacheService.GetUserBalance(c.Request.Context(), authSubject.UserID)
		if err != nil || balance < price {
			response.Error(c, http.StatusPaymentRequired, "insufficient balance")
			return
		}
	}

	file, header, err := c.Request.FormFile("image")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "image file is required")
		return
	}
	defer file.Close()

	var proxyBuf bytes.Buffer
	proxyWriter := multipart.NewWriter(&proxyBuf)
	proxyWriter.WriteField("prompt", c.PostForm("prompt"))
	proxyWriter.WriteField("aspect_ratio", c.PostForm("aspect_ratio"))
	imgPart, _ := proxyWriter.CreateFormFile("image", header.Filename)
	io.Copy(imgPart, file)
	proxyWriter.Close()

	proxyReq, _ := http.NewRequestWithContext(c.Request.Context(), "POST",
		proxyURL+"/v1/video/img2video", &proxyBuf)
	proxyReq.Header.Set("Content-Type", proxyWriter.FormDataContentType())
	if proxyToken != "" {
		proxyReq.Header.Set("Authorization", "Bearer "+proxyToken)
	}

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		response.Error(c, http.StatusBadGateway, "proxy error: "+err.Error())
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 && price > 0 {
		go func() {
			_, _ = h.usageService.Create(context.Background(), service.CreateUsageLogRequest{
				UserID:         authSubject.UserID,
				RequestID:      uuid.New().String(),
				Model:          "img2video",
				TotalCost:      price,
				ActualCost:     price,
				RateMultiplier: 1.0,
			})
		}()
	}

	c.Data(resp.StatusCode, "application/json", respBody)
}

// VideoProxy handles GET /api/v1/video/proxy — streaming proxy for video playback/download
func (h *VideoHandler) VideoProxy(c *gin.Context) {
	videoURL := c.Query("url")
	if videoURL == "" {
		response.Error(c, http.StatusBadRequest, "url parameter required")
		return
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), "GET", videoURL, nil)
	if err != nil {
		response.Error(c, http.StatusBadGateway, "invalid url")
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		response.Error(c, http.StatusBadGateway, "failed to fetch video")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		response.Error(c, http.StatusBadGateway, "upstream returned "+resp.Status)
		return
	}

	fn := c.Query("fn")
	if fn == "" {
		fn = "video.mp4"
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "video/mp4"
	}

	if c.Query("dl") == "1" {
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fn))
	}
	if resp.ContentLength > 0 {
		c.Header("Content-Length", fmt.Sprintf("%d", resp.ContentLength))
	}
	c.Status(http.StatusOK)
	c.Header("Content-Type", ct)
	io.Copy(c.Writer, resp.Body)
}
