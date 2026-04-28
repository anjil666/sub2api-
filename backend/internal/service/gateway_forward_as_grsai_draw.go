package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

type grsaiDrawRequest struct {
	Model        string   `json:"model"`
	Prompt       string   `json:"prompt"`
	AspectRatio  string   `json:"aspectRatio,omitempty"`
	URLs         []string `json:"urls,omitempty"`
	ShutProgress bool     `json:"shutProgress"`
}

func sizeToAspectRatio(size string) string {
	switch size {
	case "1024x1024":
		return "1:1"
	case "1024x1536", "1024x1792":
		return "2:3"
	case "1536x1024", "1792x1024":
		return "3:2"
	case "1024x1820":
		return "9:16"
	case "1820x1024":
		return "16:9"
	default:
		return "auto"
	}
}

// extractImageURLsWithR2 从请求体提取参考图 URL。
// base64 图片会上传到 R2 中转，返回公开 URL；纯 URL 直接透传。
// 返回 urls 列表和需要清理的 R2 key 列表。
func (s *GatewayService) extractImageURLsWithR2(ctx context.Context, body []byte) (urls []string, r2Keys []string) {
	imageField := gjson.GetBytes(body, "image")
	if !imageField.Exists() {
		return nil, nil
	}

	var items []gjson.Result
	if imageField.IsArray() {
		items = imageField.Array()
	} else {
		items = []gjson.Result{imageField}
	}

	for _, item := range items {
		val := item.String()
		if strings.HasPrefix(val, "http://") || strings.HasPrefix(val, "https://") {
			urls = append(urls, val)
			continue
		}
		// 尝试作为 base64 上传到 R2
		u, key := s.uploadBase64ToR2(ctx, val)
		if u != "" {
			urls = append(urls, u)
			r2Keys = append(r2Keys, key)
		}
	}
	return urls, r2Keys
}

func (s *GatewayService) uploadBase64ToR2(ctx context.Context, raw string) (publicURL, r2Key string) {
	if s.backupService == nil {
		return "", ""
	}
	// 去掉 data URI 前缀
	b64 := raw
	contentType := "image/png"
	if idx := strings.Index(raw, ";base64,"); idx >= 0 {
		prefix := raw[:idx]
		contentType = strings.TrimPrefix(prefix, "data:")
		b64 = raw[idx+8:]
	}

	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		// 尝试 RawStdEncoding
		data, err = base64.RawStdEncoding.DecodeString(b64)
		if err != nil {
			return "", ""
		}
	}

	store, cfg, err := s.backupService.GetObjectStoreForTempUpload(ctx)
	if err != nil {
		logger.L().Warn("grsai: R2 not configured, skipping base64 image", zap.Error(err))
		return "", ""
	}

	b := make([]byte, 8)
	_, _ = rand.Read(b)
	ext := ".png"
	if strings.Contains(contentType, "jpeg") || strings.Contains(contentType, "jpg") {
		ext = ".jpg"
	} else if strings.Contains(contentType, "webp") {
		ext = ".webp"
	}
	key := "temp/" + hex.EncodeToString(b) + ext

	if _, err := store.Upload(ctx, key, bytes.NewReader(data), contentType); err != nil {
		logger.L().Warn("grsai: failed to upload to R2", zap.Error(err))
		return "", ""
	}

	// 构建公开 URL：用 R2 公共开发 URL
	_ = cfg // cfg.Bucket 用于构建 URL
	publicURL = "https://pub-587ca1e489e540c7beeb44d9e7ca281d.r2.dev/" + key
	return publicURL, key
}

func (s *GatewayService) cleanupR2Keys(ctx context.Context, keys []string) {
	if len(keys) == 0 || s.backupService == nil {
		return
	}
	store, _, err := s.backupService.GetObjectStoreForTempUpload(ctx)
	if err != nil {
		return
	}
	for _, key := range keys {
		_ = store.Delete(ctx, key)
	}
}

// ForwardAsGrsaiDraw 将 OpenAI /v1/images/generations 请求转换为 grsai 异步做图 API。
// 流程：POST /v1/draw/completions → 拿 taskId → 轮询 POST /v1/draw/result → 包装成 OpenAI 格式返回。
func (s *GatewayService) ForwardAsGrsaiDraw(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	startTime time.Time,
) (*ForwardResult, error) {
	originalModel := gjson.GetBytes(body, "model").String()
	mappedModel := account.GetMappedModel(originalModel)

	baseURL := strings.TrimRight(account.GetCredential("base_url"), "/")
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid upstream base_url: %w", err)
	}
	apiKey := account.GetCredential("api_key")
	if apiKey == "" {
		return nil, fmt.Errorf("api_key not found in credentials")
	}

	// 提取参考图 URL（base64 自动上传到 R2 中转）
	imageURLs, r2Keys := s.extractImageURLsWithR2(ctx, body)
	if len(r2Keys) > 0 {
		defer s.cleanupR2Keys(ctx, r2Keys)
	}

	logger.L().Info("grsai draw: image extraction result",
		zap.Int64("account_id", account.ID),
		zap.Int("image_url_count", len(imageURLs)),
		zap.Strings("image_urls", imageURLs),
		zap.Int("r2_key_count", len(r2Keys)),
	)

	// 构建 grsai 请求体
	grsaiReq := grsaiDrawRequest{
		Model:        mappedModel,
		Prompt:       gjson.GetBytes(body, "prompt").String(),
		AspectRatio:  sizeToAspectRatio(gjson.GetBytes(body, "size").String()),
		URLs:         imageURLs,
		ShutProgress: true,
	}
	reqBody, _ := json.Marshal(grsaiReq)

	logger.L().Info("grsai draw: submitting task",
		zap.Int64("account_id", account.ID),
		zap.String("model", mappedModel),
		zap.String("submit_url", validatedURL+"/v1/draw/completions"),
		zap.Int("req_body_len", len(reqBody)),
	)

	// 1. 提交做图任务
	submitURL := validatedURL + "/v1/draw/completions"
	submitReq, err := http.NewRequestWithContext(ctx, "POST", submitURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("build grsai submit request: %w", err)
	}
	submitReq.Header.Set("Content-Type", "application/json")
	submitReq.Header.Set("Authorization", "Bearer "+apiKey)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	submitResp, err := s.httpUpstream.DoWithTLS(submitReq, proxyURL, account.ID, account.Concurrency, s.tlsFPProfileService.ResolveTLSProfile(account))
	if err != nil {
		if submitResp != nil && submitResp.Body != nil {
			_ = submitResp.Body.Close()
		}
		writeGatewayCCError(c, http.StatusBadGateway, "server_error", "Upstream request failed")
		return nil, fmt.Errorf("grsai submit failed: %w", err)
	}
	defer func() { _ = submitResp.Body.Close() }()

	submitBody, _ := io.ReadAll(io.LimitReader(submitResp.Body, 2<<20))

	logger.L().Info("grsai draw: upstream response",
		zap.Int64("account_id", account.ID),
		zap.Int("status_code", submitResp.StatusCode),
		zap.String("response_body", string(submitBody[:min(len(submitBody), 500)])),
	)

	if submitResp.StatusCode >= 400 {
		upstreamMsg := sanitizeUpstreamErrorMessage(strings.TrimSpace(string(submitBody)))
		if s.shouldFailoverUpstreamError(submitResp.StatusCode) {
			return nil, &UpstreamFailoverError{StatusCode: submitResp.StatusCode, ResponseBody: submitBody}
		}
		writeGatewayCCError(c, mapUpstreamStatusCode(submitResp.StatusCode), "server_error", upstreamMsg)
		return nil, fmt.Errorf("grsai submit error: %d %s", submitResp.StatusCode, upstreamMsg)
	}

	// shutProgress=true 时 grsai 返回 SSE 流，最后一条 data: 行包含最终结果
	// 解析 SSE 响应，提取最终结果
	var lastData string
	for _, line := range strings.Split(string(submitBody), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			lastData = strings.TrimPrefix(line, "data: ")
		} else if strings.HasPrefix(line, "data:") {
			lastData = strings.TrimPrefix(line, "data:")
		}
	}
	if lastData == "" {
		// 兜底：尝试直接作为 JSON 解析（非 SSE 格式）
		lastData = strings.TrimSpace(string(submitBody))
	}

	taskID := gjson.Get(lastData, "id").String()
	status := gjson.Get(lastData, "status").String()

	if status == "failed" {
		reason := gjson.Get(lastData, "failure_reason").String()
		if reason == "" {
			reason = gjson.Get(lastData, "error").String()
		}
		if reason == "" {
			reason = "unknown error"
		}
		writeGatewayCCError(c, http.StatusBadGateway, "server_error", "grsai: "+reason)
		return nil, fmt.Errorf("grsai task failed: %s", reason)
	}

	type grsaiSSEResult struct {
		ID      string `json:"id"`
		Status  string `json:"status"`
		Results []struct {
			URL string `json:"url"`
		} `json:"results"`
		FailureReason string `json:"failure_reason"`
		Error         string `json:"error"`
	}
	var result grsaiSSEResult
	if err := json.Unmarshal([]byte(lastData), &result); err != nil {
		writeGatewayCCError(c, http.StatusBadGateway, "server_error", "grsai: failed to parse response")
		return nil, fmt.Errorf("grsai: parse error: %w, body: %s", err, string(submitBody))
	}

	if result.Status != "succeeded" || len(result.Results) == 0 {
		reason := result.FailureReason
		if reason == "" {
			reason = result.Error
		}
		if reason == "" {
			reason = "no results returned (status: " + result.Status + ")"
		}
		writeGatewayCCError(c, http.StatusBadGateway, "server_error", "grsai: "+reason)
		return nil, fmt.Errorf("grsai task not succeeded: %s", lastData)
	}

	// 3. 包装成 OpenAI /v1/images/generations 响应格式
	openaiData := make([]map[string]string, 0, len(result.Results))
	for _, r := range result.Results {
		openaiData = append(openaiData, map[string]string{"url": r.URL})
	}
	openaiResp := map[string]any{
		"created": time.Now().Unix(),
		"data":    openaiData,
	}
	respJSON, _ := json.Marshal(openaiResp)
	respJSON = convertImageURLsToBase64(respJSON)
	c.Data(http.StatusOK, "application/json", respJSON)

	imageCount := int(gjson.GetBytes(body, "n").Int())
	if imageCount <= 0 {
		imageCount = len(result.Results)
	}
	if imageCount <= 0 {
		imageCount = 1
	}
	imageSize := parseOpenAIImageSize(gjson.GetBytes(body, "size").String())

	upstreamModel := ""
	if mappedModel != originalModel {
		upstreamModel = mappedModel
	}
	return &ForwardResult{
		RequestID:     taskID,
		Model:         originalModel,
		UpstreamModel: upstreamModel,
		Stream:        false,
		Duration:      time.Since(startTime),
		ImageCount:    imageCount,
		ImageSize:     imageSize,
	}, nil
}
