package service

import (
	"bytes"
	"context"
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

// ForwardAsImageGenerations proxies an OpenAI /v1/images/generations request
// directly to the upstream provider. The request body is forwarded as-is.
func (s *GatewayService) ForwardAsImageGenerations(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	startTime time.Time,
) (*ForwardResult, error) {
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

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		writeGatewayCCError(c, http.StatusBadGateway, "server_error", "Failed to read upstream response")
		return nil, fmt.Errorf("read upstream response: %w", err)
	}

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
