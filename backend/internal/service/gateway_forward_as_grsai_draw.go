package service

import (
	"bytes"
	"context"
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

type grsaiDrawResultRequest struct {
	ID string `json:"id"`
}

type grsaiDrawResultResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		ID            string `json:"id"`
		Progress      int    `json:"progress"`
		Status        string `json:"status"`
		FailureReason string `json:"failure_reason"`
		Error         string `json:"error"`
		Results       []struct {
			URL string `json:"url"`
		} `json:"results"`
	} `json:"data"`
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

func extractImageURLs(body []byte) []string {
	imageField := gjson.GetBytes(body, "image")
	if !imageField.Exists() {
		return nil
	}
	var urls []string
	if imageField.IsArray() {
		for _, item := range imageField.Array() {
			u := extractSingleImageURL(item)
			if u != "" {
				urls = append(urls, u)
			}
		}
	} else {
		u := extractSingleImageURL(imageField)
		if u != "" {
			urls = append(urls, u)
		}
	}
	return urls
}

func extractSingleImageURL(val gjson.Result) string {
	s := val.String()
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return s
	}
	// base64 data URI — grsai 不支持，跳过
	return ""
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

	// 构建 grsai 请求体
	grsaiReq := grsaiDrawRequest{
		Model:        mappedModel,
		Prompt:       gjson.GetBytes(body, "prompt").String(),
		AspectRatio:  sizeToAspectRatio(gjson.GetBytes(body, "size").String()),
		URLs:         extractImageURLs(body),
		ShutProgress: true,
	}
	reqBody, _ := json.Marshal(grsaiReq)

	logger.L().Debug("grsai draw: submitting task",
		zap.Int64("account_id", account.ID),
		zap.String("model", mappedModel),
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

	if submitResp.StatusCode >= 400 {
		upstreamMsg := sanitizeUpstreamErrorMessage(strings.TrimSpace(string(submitBody)))
		if s.shouldFailoverUpstreamError(submitResp.StatusCode) {
			return nil, &UpstreamFailoverError{StatusCode: submitResp.StatusCode, ResponseBody: submitBody}
		}
		writeGatewayCCError(c, mapUpstreamStatusCode(submitResp.StatusCode), "server_error", upstreamMsg)
		return nil, fmt.Errorf("grsai submit error: %d %s", submitResp.StatusCode, upstreamMsg)
	}

	// 从提交响应中提取 taskId
	taskID := gjson.GetBytes(submitBody, "data.id").String()
	if taskID == "" {
		taskID = gjson.GetBytes(submitBody, "id").String()
	}
	if taskID == "" {
		writeGatewayCCError(c, http.StatusBadGateway, "server_error", "grsai: no task id in response")
		return nil, fmt.Errorf("grsai: no task id, body: %s", string(submitBody))
	}

	// 2. 轮询结果（间隔 3 秒，超时 5 分钟）
	pollURL := validatedURL + "/v1/draw/result"
	pollBody, _ := json.Marshal(grsaiDrawResultRequest{ID: taskID})
	deadline := time.Now().Add(5 * time.Minute)

	var result grsaiDrawResultResponse
	for {
		if time.Now().After(deadline) {
			writeGatewayCCError(c, http.StatusGatewayTimeout, "server_error", "grsai: task timed out")
			return nil, fmt.Errorf("grsai: task %s timed out", taskID)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(3 * time.Second):
		}

		pollReq, _ := http.NewRequestWithContext(ctx, "POST", pollURL, bytes.NewReader(pollBody))
		pollReq.Header.Set("Content-Type", "application/json")
		pollReq.Header.Set("Authorization", "Bearer "+apiKey)

		pollResp, err := s.httpUpstream.DoWithTLS(pollReq, proxyURL, account.ID, account.Concurrency, s.tlsFPProfileService.ResolveTLSProfile(account))
		if err != nil {
			if pollResp != nil && pollResp.Body != nil {
				_ = pollResp.Body.Close()
			}
			continue
		}
		respBytes, _ := io.ReadAll(io.LimitReader(pollResp.Body, 2<<20))
		_ = pollResp.Body.Close()

		if err := json.Unmarshal(respBytes, &result); err != nil {
			continue
		}
		if result.Data.Status == "succeeded" {
			break
		}
		if result.Data.Status == "failed" {
			reason := result.Data.FailureReason
			if reason == "" {
				reason = result.Data.Error
			}
			writeGatewayCCError(c, http.StatusBadGateway, "server_error", "grsai: "+reason)
			return nil, fmt.Errorf("grsai task failed: %s", reason)
		}
	}

	// 3. 包装成 OpenAI /v1/images/generations 响应格式
	openaiData := make([]map[string]string, 0, len(result.Data.Results))
	for _, r := range result.Data.Results {
		openaiData = append(openaiData, map[string]string{"url": r.URL})
	}
	openaiResp := map[string]any{
		"created": time.Now().Unix(),
		"data":    openaiData,
	}
	respJSON, _ := json.Marshal(openaiResp)
	c.Data(http.StatusOK, "application/json", respJSON)

	imageCount := int(gjson.GetBytes(body, "n").Int())
	if imageCount <= 0 {
		imageCount = len(result.Data.Results)
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
