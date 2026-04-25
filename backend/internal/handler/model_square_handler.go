// Package handler provides HTTP request handlers for the application.
package handler

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

var msPerRequestTagRe = regexp.MustCompile(`\[per_request:([\d.]+)\]`)

// ModelSquareHandler handles model square (模型广场) requests
type ModelSquareHandler struct {
	gatewayService *service.GatewayService
	pricingService *service.PricingService
	apiKeyService  *service.APIKeyService
}

// NewModelSquareHandler creates a new ModelSquareHandler
func NewModelSquareHandler(
	gatewayService *service.GatewayService,
	pricingService *service.PricingService,
	apiKeyService *service.APIKeyService,
) *ModelSquareHandler {
	return &ModelSquareHandler{
		gatewayService: gatewayService,
		pricingService: pricingService,
		apiKeyService:  apiKeyService,
	}
}

// modelSquareItem represents a single model's information
type modelSquareItem struct {
	ModelName                 string   `json:"model_name"`
	Provider                  string   `json:"provider"`
	Mode                      string   `json:"mode"`
	InputPricePerMillion      float64  `json:"input_price_per_million"`
	OutputPricePerMillion     float64  `json:"output_price_per_million"`
	CacheWritePricePerMillion float64  `json:"cache_write_price_per_million"`
	CacheReadPricePerMillion  float64  `json:"cache_read_price_per_million"`
	SupportsPromptCaching     bool     `json:"supports_prompt_caching"`
	HasPricing                bool     `json:"has_pricing"`
	BillingMode               string   `json:"billing_mode,omitempty"`
	PerRequestPrice           *float64 `json:"per_request_price,omitempty"`
}

// modelSquareGroup represents models available in a specific group
type modelSquareGroup struct {
	GroupID        int64             `json:"group_id"`
	GroupName      string            `json:"group_name"`
	Platform       string            `json:"platform"`
	RateMultiplier float64           `json:"rate_multiplier"`
	BillingDisplay string            `json:"billing_display,omitempty"`
	ImagePrice1K   *float64          `json:"image_price_1k,omitempty"`
	ImagePrice2K   *float64          `json:"image_price_2k,omitempty"`
	ImagePrice4K        *float64          `json:"image_price_4k,omitempty"`
	ImageStudioEnabled  bool              `json:"image_studio_enabled"`
	Models              []modelSquareItem `json:"models"`
}

// List returns all models available to the current user, grouped by group
// GET /api/v1/models
func (h *ModelSquareHandler) List(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	ctx := c.Request.Context()

	// 1. Get user's available groups
	groups, err := h.apiKeyService.GetAvailableGroups(ctx, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 2. For each group, get available models and their pricing
	result := make([]modelSquareGroup, 0, len(groups))

	for _, group := range groups {
		groupID := group.ID
		models := h.gatewayService.GetAvailableModels(ctx, &groupID, group.Platform)

		var billingDisplay string
		// ImagePrice1K takes priority over per_request tag in description
		if group.ImagePrice1K != nil && *group.ImagePrice1K > 0 {
			billingDisplay = fmt.Sprintf("$%.3g/次", *group.ImagePrice1K)
		} else if m := msPerRequestTagRe.FindStringSubmatch(group.Description); len(m) == 2 {
			if price, err := strconv.ParseFloat(m[1], 64); err == nil && price > 0 {
				billingDisplay = fmt.Sprintf("$%.3g/次", price)
			}
		}

		if len(models) == 0 {
			result = append(result, modelSquareGroup{
				GroupID:            group.ID,
				GroupName:          group.Name,
				Platform:           group.Platform,
				RateMultiplier:     group.RateMultiplier,
				BillingDisplay:     billingDisplay,
				ImagePrice1K:       group.ImagePrice1K,
				ImagePrice2K:       group.ImagePrice2K,
				ImagePrice4K:       group.ImagePrice4K,
				ImageStudioEnabled: group.ImageStudioEnabled,
				Models:             []modelSquareItem{},
			})
			continue
		}

		sort.Strings(models)

		modelInfos := make([]modelSquareItem, 0, len(models))
		for _, modelName := range models {
			info := modelSquareItem{
				ModelName: modelName,
			}

			if service.IsImageModel(modelName) {
				info.HasPricing = true
				info.BillingMode = string(service.BillingModeImage)
				info.Mode = "image"
				info.Provider = inferProviderFromModelName(modelName)
				if group.ImagePrice1K != nil && *group.ImagePrice1K > 0 {
					p := *group.ImagePrice1K
					info.PerRequestPrice = &p
				} else if price, ok := service.LookupImageModelPrice(modelName); ok {
					p := price * group.RateMultiplier
					info.PerRequestPrice = &p
				} else {
					p := 0.080 * group.RateMultiplier
					info.PerRequestPrice = &p
				}
			} else {
				pricing := h.pricingService.GetModelPricing(modelName)
				info.HasPricing = pricing != nil

				if pricing != nil {
					info.Provider = pricing.LiteLLMProvider
					info.Mode = pricing.Mode
					info.SupportsPromptCaching = pricing.SupportsPromptCaching
					info.InputPricePerMillion = pricing.InputCostPerToken * 1_000_000 * group.RateMultiplier
					info.OutputPricePerMillion = pricing.OutputCostPerToken * 1_000_000 * group.RateMultiplier
					info.CacheWritePricePerMillion = pricing.CacheCreationInputTokenCost * 1_000_000 * group.RateMultiplier
					info.CacheReadPricePerMillion = pricing.CacheReadInputTokenCost * 1_000_000 * group.RateMultiplier
				} else {
					info.Provider = inferProviderFromModelName(modelName)
					info.Mode = "chat"
				}
			}

			modelInfos = append(modelInfos, info)
		}

		result = append(result, modelSquareGroup{
			GroupID:            group.ID,
			GroupName:          group.Name,
			Platform:           group.Platform,
			RateMultiplier:     group.RateMultiplier,
			BillingDisplay:     billingDisplay,
			ImagePrice1K:       group.ImagePrice1K,
			ImagePrice2K:       group.ImagePrice2K,
			ImagePrice4K:       group.ImagePrice4K,
			ImageStudioEnabled: group.ImageStudioEnabled,
			Models:             modelInfos,
		})
	}

	response.Success(c, result)
}

// inferProviderFromModelName attempts to guess the provider from the model name
func inferProviderFromModelName(modelName string) string {
	lower := strings.ToLower(modelName)
	switch {
	case strings.Contains(lower, "claude"):
		return "anthropic"
	case strings.HasPrefix(lower, "gpt") || strings.HasPrefix(lower, "o1") || strings.HasPrefix(lower, "o3") || strings.HasPrefix(lower, "o4"):
		return "openai"
	case strings.Contains(lower, "gemini"):
		return "google"
	default:
		return "unknown"
	}
}
