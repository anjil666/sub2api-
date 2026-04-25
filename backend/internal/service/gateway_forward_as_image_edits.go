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
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ForwardAsImageEdits proxies an OpenAI /v1/images/edits request
// directly to the upstream provider. The request body is multipart/form-data
// and is forwarded as-is, preserving the original Content-Type header.
func (s *GatewayService) ForwardAsImageEdits(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	contentType string,
	model string,
	startTime time.Time,
) (*ForwardResult, error) {
	// grsai 账号走异步做图适配器（将 multipart 转为 grsai JSON 格式）
	logger.L().Info("image_edits: checking site_type",
		zap.Int64("account_id", account.ID),
		zap.String("site_type", account.GetCredential("site_type")),
		zap.Any("credentials_keys", credentialKeys(account.Credentials)),
	)
	if account.GetCredential("site_type") == "grsai" {
		return s.forwardImageEditsAsGrsaiDraw(ctx, c, account, body, contentType, model, startTime)
	}

	mappedModel := account.GetMappedModel(model)

	baseURL := strings.TrimRight(account.GetCredential("base_url"), "/")
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid upstream base_url: %w", err)
	}
	targetURL := validatedURL + "/v1/images/edits"

	apiKey := account.GetCredential("api_key")
	if apiKey == "" {
		return nil, fmt.Errorf("api_key not found in credentials")
	}

	// If model was mapped, replace in the multipart body
	forwardBody := body
	if mappedModel != model {
		forwardBody = replaceMultipartField(body, contentType, "model", mappedModel)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(forwardBody))
	if err != nil {
		return nil, fmt.Errorf("build upstream request: %w", err)
	}
	// Preserve original Content-Type (multipart/form-data with boundary)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+apiKey)

	logger.L().Debug("gateway image_edits: forwarding to upstream",
		zap.Int64("account_id", account.ID),
		zap.String("target_url", targetURL),
		zap.String("model", model),
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

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		writeGatewayCCError(c, http.StatusBadGateway, "server_error", "Failed to read upstream response")
		return nil, fmt.Errorf("read upstream response: %w", err)
	}

	c.Data(resp.StatusCode, "application/json", respBody)

	// Extract billing fields from multipart form values
	imageCount := 1
	if nStr := c.Request.FormValue("n"); nStr != "" {
		if n, err := strconv.Atoi(nStr); err == nil && n > 0 {
			imageCount = n
		}
	}
	imageSize := parseOpenAIImageSize(c.Request.FormValue("size"))

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
		ImageCount:    imageCount,
		ImageSize:     imageSize,
	}, nil
}

// replaceMultipartField is a best-effort replacement for a form field value
// in a multipart body. For model mapping in image edits, this is sufficient.
func replaceMultipartField(body []byte, contentType, fieldName, newValue string) []byte {
	// Simple string replacement approach for the model field in multipart
	// The field appears as: Content-Disposition: form-data; name="model"\r\n\r\n<value>\r\n
	needle := []byte("name=\"" + fieldName + "\"\r\n\r\n")
	idx := bytes.Index(body, needle)
	if idx < 0 {
		return body
	}
	valueStart := idx + len(needle)
	valueEnd := bytes.Index(body[valueStart:], []byte("\r\n"))
	if valueEnd < 0 {
		return body
	}
	valueEnd += valueStart

	result := make([]byte, 0, len(body))
	result = append(result, body[:valueStart]...)
	result = append(result, []byte(newValue)...)
	result = append(result, body[valueEnd:]...)
	return result
}

// forwardImageEditsAsGrsaiDraw converts a multipart /v1/images/edits request
// into the JSON format expected by ForwardAsGrsaiDraw.
func (s *GatewayService) forwardImageEditsAsGrsaiDraw(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	contentType string,
	model string,
	startTime time.Time,
) (*ForwardResult, error) {
	prompt, size, images, err := parseMultipartImageEdits(body, contentType)
	if err != nil {
		writeGatewayCCError(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse multipart body")
		return nil, fmt.Errorf("parse multipart for grsai: %w", err)
	}

	imageArr := make([]any, 0, len(images))
	for _, img := range images {
		ct := http.DetectContentType(img)
		b64 := "data:" + ct + ";base64," + base64.StdEncoding.EncodeToString(img)
		imageArr = append(imageArr, b64)
	}

	jsonBody, _ := json.Marshal(map[string]any{
		"model":  model,
		"prompt": prompt,
		"size":   size,
		"n":      1,
		"image":  imageArr,
	})

	logger.L().Info("grsai image_edits: converted multipart to JSON",
		zap.Int64("account_id", account.ID),
		zap.String("prompt", prompt),
		zap.String("size", size),
		zap.Int("image_count", len(images)),
		zap.Int("json_body_len", len(jsonBody)),
	)

	return s.ForwardAsGrsaiDraw(ctx, c, account, jsonBody, startTime)
}

func parseMultipartImageEdits(body []byte, contentType string) (prompt, size string, images [][]byte, err error) {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "", "", nil, fmt.Errorf("parse content-type: %w", err)
	}
	boundary := params["boundary"]
	if boundary == "" {
		return "", "", nil, fmt.Errorf("no boundary in content-type")
	}

	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", nil, fmt.Errorf("read part: %w", err)
		}
		name := part.FormName()
		switch name {
		case "prompt":
			data, _ := io.ReadAll(part)
			prompt = strings.TrimSpace(string(data))
		case "size":
			data, _ := io.ReadAll(part)
			size = strings.TrimSpace(string(data))
		case "image":
			data, _ := io.ReadAll(part)
			if len(data) > 0 {
				images = append(images, data)
			}
		}
		_ = part.Close()
	}
	return prompt, size, images, nil
}

func credentialKeys(creds map[string]any) []string {
	keys := make([]string, 0, len(creds))
	for k := range creds {
		keys = append(keys, k)
	}
	return keys
}