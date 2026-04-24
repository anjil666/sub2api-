package handler

import (
	"bytes"
	"context"
	"errors"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ImageEdits handles OpenAI /v1/images/edits requests.
// Proxies the multipart/form-data request to the upstream provider.
func (h *GatewayHandler) ImageEdits(c *gin.Context) {
	requestStart := time.Now()

	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.chatCompletionsErrorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.chatCompletionsErrorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	reqLog := requestLogger(
		c,
		"handler.gateway.image_edits",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)

	// Read the raw body (multipart/form-data)
	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		h.chatCompletionsErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	if len(body) == 0 {
		h.chatCompletionsErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}

	contentType := c.GetHeader("Content-Type")

	// Extract model from raw multipart body (c.Request.Body is already consumed)
	reqModel := extractMultipartField(body, contentType, "model")
	reqLog.Info("gateway.image_edits.debug_model_extract",
		zap.String("content_type", contentType),
		zap.Int("body_len", len(body)),
		zap.String("extracted_model", reqModel),
	)
	if reqModel == "" {
		reqModel = "gpt-image-1"
	}
	reqLog = reqLog.With(zap.String("model", reqModel))
	setOpsRequestContext(c, reqModel, false, nil)

	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}

	subscription, _ := middleware2.GetSubscriptionFromContext(c)

	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription); err != nil {
		reqLog.Info("gateway.image_edits.billing_check_failed", zap.Error(err))
		status, code, message := billingErrorDetails(err)
		h.chatCompletionsErrorResponse(c, status, code, message)
		return
	}

	parsedReq := &service.ParsedRequest{Model: reqModel, Stream: false, Body: body}
	sessionHash := h.gatewayService.GenerateSessionHash(parsedReq)

	fs := NewFailoverState(h.maxAccountSwitches, false)

	for {
		selection, err := h.gatewayService.SelectAccountWithLoadAwareness(c.Request.Context(), apiKey.GroupID, sessionHash, reqModel, fs.FailedAccountIDs, "", int64(0))
		if err != nil {
			if len(fs.FailedAccountIDs) == 0 {
				h.chatCompletionsErrorResponse(c, http.StatusServiceUnavailable, "api_error", "No available accounts: "+err.Error())
				return
			}
			action := fs.HandleSelectionExhausted(c.Request.Context())
			switch action {
			case FailoverContinue:
				continue
			case FailoverCanceled:
				return
			default:
				h.chatCompletionsErrorResponse(c, http.StatusBadGateway, "server_error", "All available accounts exhausted")
				return
			}
		}
		account := selection.Account
		setOpsSelectedAccount(c, account.ID, account.Platform)

		accountReleaseFunc := selection.ReleaseFunc
		if !selection.Acquired {
			h.chatCompletionsErrorResponse(c, http.StatusServiceUnavailable, "api_error", "No available accounts")
			return
		}
		accountReleaseFunc = wrapReleaseOnDone(c.Request.Context(), accountReleaseFunc)

		result, err := h.gatewayService.ForwardAsImageEdits(c.Request.Context(), c, account, body, contentType, reqModel, requestStart)

		if accountReleaseFunc != nil {
			accountReleaseFunc()
		}

		if err != nil {
			var failoverErr *service.UpstreamFailoverError
			if errors.As(err, &failoverErr) {
				action := fs.HandleFailoverError(c.Request.Context(), h.gatewayService, account.ID, account.Platform, failoverErr)
				switch action {
				case FailoverContinue:
					continue
				case FailoverExhausted:
					h.handleCCFailoverExhausted(c, fs.LastFailoverErr, false)
					return
				case FailoverCanceled:
					return
				}
			}
			reqLog.Error("gateway.image_edits.forward_failed",
				zap.Int64("account_id", account.ID),
				zap.Error(err),
			)
			return
		}

		userAgent := c.GetHeader("User-Agent")
		clientIP := ip.GetClientIP(c)
		requestPayloadHash := service.HashUsageRequestPayload(body)
		inboundEndpoint := GetInboundEndpoint(c)
		upstreamEndpoint := "/v1/images/edits"

		h.submitUsageRecordTask(func(ctx context.Context) {
			if err := h.gatewayService.RecordUsage(ctx, &service.RecordUsageInput{
				Result:             result,
				APIKey:             apiKey,
				User:               apiKey.User,
				Account:            account,
				Subscription:       subscription,
				InboundEndpoint:    inboundEndpoint,
				UpstreamEndpoint:   upstreamEndpoint,
				UserAgent:          userAgent,
				IPAddress:          clientIP,
				RequestPayloadHash: requestPayloadHash,
				APIKeyService:      h.apiKeyService,
			}); err != nil {
				reqLog.Error("gateway.image_edits.record_usage_failed",
					zap.Int64("account_id", account.ID),
					zap.Error(err),
				)
			}
		})
		return
	}
}

func extractMultipartField(body []byte, contentType, fieldName string) string {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		return ""
	}
	boundary := params["boundary"]
	if boundary == "" {
		return ""
	}
	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		part, err := reader.NextPart()
		if err != nil {
			break
		}
		if part.FormName() == fieldName {
			var buf bytes.Buffer
			buf.ReadFrom(part)
			return strings.TrimSpace(buf.String())
		}
		part.Close()
	}
	return ""
}