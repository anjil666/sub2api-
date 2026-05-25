package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// ForwardAsImageGenerations proxies an OpenAI /v1/images/generations request
// directly to the upstream provider. The request body is forwarded as-is.
func (s *GatewayService) ForwardAsImageGenerations(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	startTime time.Time,
) (*ForwardResult, error) {
	// grsai 账号走异步做图适配器
	if account.GetCredential("site_type") == "grsai" {
		return s.ForwardAsGrsaiDraw(ctx, c, account, body, startTime)
	}

	// image_via_chat 标记：通过 /v1/chat/completions 做图
	if account.GetCredential("image_via_chat") == "true" {
		return s.forwardImageViaChatCompletions(ctx, c, account, body, startTime)
	}

	originalModel := gjson.GetBytes(body, "model").String()

	mappedModel := account.GetMappedModel(originalModel)
	if mappedModel != originalModel {
		body = s.ReplaceModelInBody(body, mappedModel)
	}

	baseURL := strings.TrimRight(account.GetCredential("base_url"), "/")
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid upstream base_url: %w", err)
	}
	targetURL := validatedURL + "/v1/images/generations"

	apiKey := account.GetCredential("api_key")
	if apiKey == "" {
		return nil, fmt.Errorf("api_key not found in credentials")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build upstream request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	logger.L().Debug("gateway image_generations: forwarding to upstream",
		zap.Int64("account_id", account.ID),
		zap.String("target_url", targetURL),
		zap.String("model", originalModel),
	)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, s.tlsFPProfileService.ResolveTLSProfile(account))
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		writeGatewayCCError(c, http.StatusBadGateway, "server_error", "Upstream request failed")
		return nil, fmt.Errorf("upstream request failed: %s", safeErr)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(respBody))
		upstreamMsg = sanitizeUpstreamErrorMessage(upstreamMsg)

		if s.shouldFailoverUpstreamError(resp.StatusCode) {
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:           account.Platform,
				AccountID:          account.ID,
				AccountName:        account.Name,
				UpstreamStatusCode: resp.StatusCode,
				Kind:               "failover",
				Message:            upstreamMsg,
			})
			if s.rateLimitService != nil {
				s.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody)
			}
			return nil, &UpstreamFailoverError{
				StatusCode:   resp.StatusCode,
				ResponseBody: respBody,
			}
		}

		writeGatewayCCError(c, mapUpstreamStatusCode(resp.StatusCode), "server_error", upstreamMsg)
		return nil, fmt.Errorf("upstream error: %d %s", resp.StatusCode, upstreamMsg)
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20))
	if err != nil {
		writeGatewayCCError(c, http.StatusBadGateway, "server_error", "Failed to read upstream response")
		return nil, fmt.Errorf("read upstream response: %w", err)
	}

	respBody = convertImageURLsToBase64(respBody)

	c.Data(resp.StatusCode, "application/json", respBody)

	imageCount := int(gjson.GetBytes(body, "n").Int())
	if imageCount <= 0 {
		imageCount = 1
	}

	imageSize := parseOpenAIImageSize(gjson.GetBytes(body, "size").String())

	upstreamModel := ""
	if mappedModel != originalModel {
		upstreamModel = mappedModel
	}
	return &ForwardResult{
		RequestID:     resp.Header.Get("x-request-id"),
		Model:         originalModel,
		UpstreamModel: upstreamModel,
		Stream:        false,
		Duration:      time.Since(startTime),
		ImageCount:    imageCount,
		ImageSize:     imageSize,
	}, nil
}

// parseOpenAIImageSize maps OpenAI size strings to billing tiers.
func parseOpenAIImageSize(size string) string {
	switch strings.ToLower(size) {
	case "1024x1024", "512x512", "256x256":
		return "1K"
	case "1024x1536", "1536x1024", "2048x2048":
		return "2K"
	case "3840x2160", "2160x3840":
		return "4K"
	default:
		if size == "" {
			return "1K"
		}
		return "1K"
	}
}

func convertImageURLsToBase64(respBody []byte) []byte {
	dataArr := gjson.GetBytes(respBody, "data")
	if !dataArr.IsArray() {
		return respBody
	}
	needConvert := false
	for _, item := range dataArr.Array() {
		if item.Get("url").Exists() && !item.Get("b64_json").Exists() {
			needConvert = true
			break
		}
	}
	if !needConvert {
		return respBody
	}
	var parsed map[string]any
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return respBody
	}
	dataSlice, ok := parsed["data"].([]any)
	if !ok {
		return respBody
	}
	client := &http.Client{Timeout: 180 * time.Second}
	for _, item := range dataSlice {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		urlVal, hasURL := m["url"].(string)
		if !hasURL || urlVal == "" {
			continue
		}
		if _, hasB64 := m["b64_json"]; hasB64 {
			continue
		}
		req, err := http.NewRequest("GET", urlVal, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		imgData, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
		resp.Body.Close()
		if err != nil || len(imgData) == 0 || resp.StatusCode != 200 {
			continue
		}
		contentType := resp.Header.Get("Content-Type")
		if contentType != "" && !strings.HasPrefix(contentType, "image/") {
			continue
		}
		m["b64_json"] = base64.StdEncoding.EncodeToString(imgData)
		delete(m, "url")
	}
	out, err := json.Marshal(parsed)
	if err != nil {
		return respBody
	}
	return out
}

// forwardImageViaChatCompletions converts an image generation request to chat completions format
// and forwards it to the upstream /v1/chat/completions endpoint.
func (s *GatewayService) forwardImageViaChatCompletions(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	startTime time.Time,
) (*ForwardResult, error) {
	originalModel := gjson.GetBytes(body, "model").String()
	prompt := gjson.GetBytes(body, "prompt").String()
	size := gjson.GetBytes(body, "size").String()
	quality := gjson.GetBytes(body, "quality").String()
	imageCount := int(gjson.GetBytes(body, "n").Int())
	if imageCount <= 0 {
		imageCount = 1
	}

	mappedModel := account.GetMappedModel(originalModel)

	// Build chat completions request
	chatReq := map[string]any{
		"model": mappedModel,
		"messages": []map[string]any{
			{"role": "user", "content": prompt},
		},
		"modalities": []string{"text", "image"},
	}
	if size != "" {
		chatReq["size"] = size
	}
	if quality != "" {
		chatReq["quality"] = quality
	}

	chatBody, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal chat request: %w", err)
	}

	baseURL := strings.TrimRight(account.GetCredential("base_url"), "/")
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid upstream base_url: %w", err)
	}
	targetURL := validatedURL + "/v1/chat/completions"

	apiKey := account.GetCredential("api_key")
	if apiKey == "" {
		return nil, fmt.Errorf("api_key not found in credentials")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(chatBody))
	if err != nil {
		return nil, fmt.Errorf("build upstream request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	logger.L().Debug("gateway image_via_chat: forwarding to upstream",
		zap.Int64("account_id", account.ID),
		zap.String("target_url", targetURL),
		zap.String("model", originalModel),
	)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, s.tlsFPProfileService.ResolveTLSProfile(account))
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		writeGatewayCCError(c, http.StatusBadGateway, "server_error", "Upstream request failed")
		return nil, fmt.Errorf("upstream request failed: %s", sanitizeUpstreamErrorMessage(err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		upstreamMsg := sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(respBody)))
		if s.shouldFailoverUpstreamError(resp.StatusCode) {
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:           account.Platform,
				AccountID:          account.ID,
				AccountName:        account.Name,
				UpstreamStatusCode: resp.StatusCode,
				Kind:               "failover",
				Message:            upstreamMsg,
			})
			if s.rateLimitService != nil {
				s.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody)
			}
			return nil, &UpstreamFailoverError{StatusCode: resp.StatusCode, ResponseBody: respBody}
		}
		writeGatewayCCError(c, mapUpstreamStatusCode(resp.StatusCode), "server_error", upstreamMsg)
		return nil, fmt.Errorf("upstream error: %d %s", resp.StatusCode, upstreamMsg)
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20))
	if err != nil {
		writeGatewayCCError(c, http.StatusBadGateway, "server_error", "Failed to read upstream response")
		return nil, fmt.Errorf("read upstream response: %w", err)
	}

	// Extract image from chat completions response and convert to image generations format
	imageData := extractImageFromChatResponse(respBody)
	if imageData == "" {
		// Log full response for debugging (truncated)
		respSnippet := string(respBody)
		if len(respSnippet) > 1000 {
			respSnippet = respSnippet[:1000]
		}
		logger.L().Error("image_via_chat: no image found in upstream response",
			zap.String("response_snippet", respSnippet),
			zap.String("model", originalModel),
		)
		// No image found — extract upstream text content for diagnostics
		upstreamText := gjson.GetBytes(respBody, "choices.0.message.content").String()
		if len(upstreamText) > 200 {
			upstreamText = upstreamText[:200]
		}
		errMsg := "No image in upstream response"
		if upstreamText != "" {
			errMsg = fmt.Sprintf("No image in upstream response. Upstream replied: %s", upstreamText)
		}
		writeGatewayCCError(c, http.StatusBadGateway, "server_error", errMsg)
		return nil, fmt.Errorf("no image in chat response, upstream text: %s", upstreamText)
	}

	// Build OpenAI image generations response format
	imgResp := map[string]any{
		"created": time.Now().Unix(),
		"data": []map[string]any{
			{"b64_json": imageData},
		},
	}
	imgRespBody, _ := json.Marshal(imgResp)
	c.Data(http.StatusOK, "application/json", imgRespBody)

	imageSize := parseOpenAIImageSize(size)
	upstreamModel := ""
	if mappedModel != originalModel {
		upstreamModel = mappedModel
	}
	return &ForwardResult{
		RequestID:     resp.Header.Get("x-request-id"),
		Model:         originalModel,
		UpstreamModel: upstreamModel,
		Stream:        false,
		Duration:      time.Since(startTime),
		ImageCount:    imageCount,
		ImageSize:     imageSize,
	}, nil
}

// extractImageFromChatResponse extracts base64 image data from a chat completions response.
// Supports both inline base64 data URLs and content blocks with image_url type.
func extractImageFromChatResponse(respBody []byte) string {
	// Try choices[0].message.content as array (multimodal response)
	contents := gjson.GetBytes(respBody, "choices.0.message.content")
	if contents.IsArray() {
		for _, block := range contents.Array() {
			if block.Get("type").String() == "image_url" {
				url := block.Get("image_url.url").String()
				if b64 := extractBase64FromDataURL(url); b64 != "" {
					return b64
				}
				// If it's a regular URL, download and convert
				if strings.HasPrefix(url, "http") {
					if b64 := downloadImageAsBase64(url); b64 != "" {
						return b64
					}
				}
			}
		}
	}

	// Try choices[0].message.content as string (may contain markdown image)
	contentStr := gjson.GetBytes(respBody, "choices.0.message.content").String()
	if b64 := extractBase64FromDataURL(contentStr); b64 != "" {
		return b64
	}

	// Try to find image URL in content string (markdown format)
	if idx := strings.Index(contentStr, "!["); idx >= 0 {
		if start := strings.Index(contentStr[idx:], "("); start >= 0 {
			if end := strings.Index(contentStr[idx+start:], ")"); end >= 0 {
				url := contentStr[idx+start+1 : idx+start+end]
				if strings.HasPrefix(url, "http") {
					if b64 := downloadImageAsBase64(url); b64 != "" {
						return b64
					}
				}
			}
		}
	}

	return ""
}

func extractBase64FromDataURL(s string) string {
	prefix := "data:image/"
	if idx := strings.Index(s, prefix); idx >= 0 {
		if b64Idx := strings.Index(s[idx:], ";base64,"); b64Idx >= 0 {
			start := idx + b64Idx + len(";base64,")
			// Find end of base64 (next quote, space, or end)
			end := len(s)
			for i := start; i < len(s); i++ {
				if s[i] == '"' || s[i] == '\'' || s[i] == ')' || s[i] == ' ' || s[i] == '\n' {
					end = i
					break
				}
			}
			return s[start:end]
		}
	}
	return ""
}

func downloadImageAsBase64(url string) string {
	client := &http.Client{Timeout: 180 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil || len(data) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(data)
}

// forwardImageEditsViaChatCompletions converts a multipart /v1/images/edits request
// into a chat completions request with image content blocks.
func (s *GatewayService) forwardImageEditsViaChatCompletions(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	contentType string,
	model string,
	startTime time.Time,
) (*ForwardResult, error) {
	prompt, size, quality, images, err := parseMultipartImageEditsWithQuality(body, contentType)
	if err != nil {
		writeGatewayCCError(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse multipart body")
		return nil, fmt.Errorf("parse multipart for image_via_chat: %w", err)
	}

	mappedModel := account.GetMappedModel(model)

	// Build content array: text prompt + image(s)
	contentParts := []map[string]any{
		{"type": "text", "text": prompt},
	}
	for _, img := range images {
		ct := http.DetectContentType(img)
		dataURL := "data:" + ct + ";base64," + base64.StdEncoding.EncodeToString(img)
		contentParts = append(contentParts, map[string]any{
			"type": "image_url",
			"image_url": map[string]any{
				"url": dataURL,
			},
		})
	}

	chatReq := map[string]any{
		"model": mappedModel,
		"messages": []map[string]any{
			{"role": "user", "content": contentParts},
		},
		"modalities": []string{"text", "image"},
	}
	if size != "" {
		chatReq["size"] = size
	}
	if quality != "" {
		chatReq["quality"] = quality
	}

	chatBody, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal chat request: %w", err)
	}

	baseURL := strings.TrimRight(account.GetCredential("base_url"), "/")
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid upstream base_url: %w", err)
	}
	targetURL := validatedURL + "/v1/chat/completions"

	apiKey := account.GetCredential("api_key")
	if apiKey == "" {
		return nil, fmt.Errorf("api_key not found in credentials")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(chatBody))
	if err != nil {
		return nil, fmt.Errorf("build upstream request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	logger.L().Debug("gateway image_edits_via_chat: forwarding to upstream",
		zap.Int64("account_id", account.ID),
		zap.String("target_url", targetURL),
		zap.String("model", model),
		zap.Int("image_count", len(images)),
	)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, s.tlsFPProfileService.ResolveTLSProfile(account))
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		writeGatewayCCError(c, http.StatusBadGateway, "server_error", "Upstream request failed")
		return nil, fmt.Errorf("upstream request failed: %s", sanitizeUpstreamErrorMessage(err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		writeGatewayCCError(c, mapUpstreamStatusCode(resp.StatusCode), "server_error", extractUpstreamErrorMessage(respBody))
		return nil, fmt.Errorf("upstream error: %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20))
	if err != nil {
		writeGatewayCCError(c, http.StatusBadGateway, "server_error", "Failed to read upstream response")
		return nil, fmt.Errorf("read upstream response: %w", err)
	}

	imageData := extractImageFromChatResponse(respBody)
	if imageData == "" {
		upstreamText := gjson.GetBytes(respBody, "choices.0.message.content").String()
		if len(upstreamText) > 200 {
			upstreamText = upstreamText[:200]
		}
		errMsg := "No image in upstream response"
		if upstreamText != "" {
			errMsg = fmt.Sprintf("No image in upstream response. Upstream replied: %s", upstreamText)
		}
		writeGatewayCCError(c, http.StatusBadGateway, "server_error", errMsg)
		return nil, fmt.Errorf("no image in chat response, upstream text: %s", upstreamText)
	}

	imgResp := map[string]any{
		"created": time.Now().Unix(),
		"data": []map[string]any{
			{"b64_json": imageData},
		},
	}
	imgRespBody, _ := json.Marshal(imgResp)
	c.Data(http.StatusOK, "application/json", imgRespBody)

	imageSize := parseOpenAIImageSize(size)
	upstreamModel := ""
	if mappedModel != model {
		upstreamModel = mappedModel
	}
	return &ForwardResult{
		RequestID:     resp.Header.Get("x-request-id"),
		Model:         model,
		UpstreamModel: upstreamModel,
		Stream:        false,
		Duration:      time.Since(startTime),
		ImageCount:    1,
		ImageSize:     imageSize,
	}, nil
}

// parseMultipartImageEditsWithQuality parses a multipart image edit request body,
// extracting prompt, size, quality, and image files.
func parseMultipartImageEditsWithQuality(body []byte, contentType string) (prompt, size, quality string, images [][]byte, err error) {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("parse content-type: %w", err)
	}
	boundary := params["boundary"]
	if boundary == "" {
		return "", "", "", nil, fmt.Errorf("no boundary in content-type")
	}

	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", "", nil, fmt.Errorf("read part: %w", err)
		}
		name := part.FormName()
		switch name {
		case "prompt":
			data, _ := io.ReadAll(part)
			prompt = strings.TrimSpace(string(data))
		case "size":
			data, _ := io.ReadAll(part)
			size = strings.TrimSpace(string(data))
		case "quality":
			data, _ := io.ReadAll(part)
			quality = strings.TrimSpace(string(data))
		case "image", "image[]":
			data, _ := io.ReadAll(part)
			if len(data) > 0 {
				images = append(images, data)
			}
		}
		_ = part.Close()
	}
	return prompt, size, quality, images, nil
}
