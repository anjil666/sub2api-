package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// UpstreamSyncService 上游站点同步服务
type UpstreamSyncService struct {
	siteRepo       UpstreamSiteRepository
	groupRepo      GroupRepository
	adminService   AdminService
	channelService *ChannelService

	httpClient *http.Client
	stopCh     chan struct{}
	stopOnce   sync.Once
	wg         sync.WaitGroup
}

// NewUpstreamSyncService 创建上游同步服务
func NewUpstreamSyncService(
	siteRepo UpstreamSiteRepository,
	groupRepo GroupRepository,
	adminService AdminService,
	channelService *ChannelService,
) *UpstreamSyncService {
	return &UpstreamSyncService{
		siteRepo:       siteRepo,
		groupRepo:      groupRepo,
		adminService:   adminService,
		channelService: channelService,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		stopCh: make(chan struct{}),
	}
}

// Start 启动后台定时同步
func (s *UpstreamSyncService) Start() {
	if s == nil {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.checkAndSync()
			case <-s.stopCh:
				return
			}
		}
	}()
	log.Println("[UpstreamSync] Background sync started (1 min interval)")
}

// Stop 停止后台同步
func (s *UpstreamSyncService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() { close(s.stopCh) })
	s.wg.Wait()
	log.Println("[UpstreamSync] Background sync stopped")
}

// checkAndSync 检查并同步到期的站点
func (s *UpstreamSyncService) checkAndSync() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	sites, err := s.siteRepo.ListDueForSync(ctx)
	if err != nil {
		log.Printf("[UpstreamSync] Failed to list due sites: %v", err)
		return
	}
	if len(sites) == 0 {
		return
	}

	log.Printf("[UpstreamSync] Found %d site(s) due for sync", len(sites))
	for i := range sites {
		select {
		case <-s.stopCh:
			return
		default:
		}

		site := &sites[i]
		result := s.syncSite(ctx, site)
		if result.Error != "" {
			log.Printf("[UpstreamSync] Site %q (#%d) sync failed: %s", site.Name, site.ID, result.Error)
			_ = s.siteRepo.UpdateSyncStatus(ctx, site.ID, "error", result.Error, 0)
		} else {
			log.Printf("[UpstreamSync] Site %q (#%d) sync success: %d models", site.Name, site.ID, result.ModelsDiscovered)
			_ = s.siteRepo.UpdateSyncStatus(ctx, site.ID, "success", "", result.ModelsDiscovered)
		}
	}
}

// SyncSiteNow 手动立即同步指定站点
func (s *UpstreamSyncService) SyncSiteNow(ctx context.Context, siteID int64) (*SyncResult, error) {
	site, err := s.siteRepo.GetByID(ctx, siteID)
	if err != nil {
		return nil, err
	}

	result := s.syncSite(ctx, site)

	if result.Error != "" {
		_ = s.siteRepo.UpdateSyncStatus(ctx, site.ID, "error", result.Error, 0)
		return &result, fmt.Errorf("sync failed: %s", result.Error)
	}
	_ = s.siteRepo.UpdateSyncStatus(ctx, site.ID, "success", "", result.ModelsDiscovered)
	return &result, nil
}

// FetchUpstreamModels 预览上游模型列表（不创建资源）
func (s *UpstreamSyncService) FetchUpstreamModels(ctx context.Context, siteID int64) ([]UpstreamModelInfo, error) {
	site, err := s.siteRepo.GetByID(ctx, siteID)
	if err != nil {
		return nil, err
	}
	return s.fetchModels(ctx, site)
}

// CheckUpstreamBalance 查询上游余额
func (s *UpstreamSyncService) CheckUpstreamBalance(ctx context.Context, siteID int64) (*UpstreamBalanceInfo, error) {
	site, err := s.siteRepo.GetByID(ctx, siteID)
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(site.BaseURL, "/") + "/v1/usage"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+site.APIKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch balance: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("upstream returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data UpstreamBalanceInfo `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode balance: %w", err)
	}
	return &result.Data, nil
}

// ── 核心同步逻辑 ──

func (s *UpstreamSyncService) syncSite(ctx context.Context, site *UpstreamSite) SyncResult {
	// 1. 获取上游模型列表
	models, err := s.fetchModels(ctx, site)
	if err != nil {
		return SyncResult{Error: fmt.Sprintf("fetch models: %v", err)}
	}
	if len(models) == 0 {
		return SyncResult{Error: "upstream returned 0 models"}
	}

	// 2. 确保分组存在
	groupID, err := s.ensureGroup(ctx, site)
	if err != nil {
		return SyncResult{Error: fmt.Sprintf("ensure group: %v", err)}
	}

	// 3. 确保账号存在（含 model_mapping）
	accountID, err := s.ensureAccount(ctx, site, groupID, models)
	if err != nil {
		return SyncResult{Error: fmt.Sprintf("ensure account: %v", err)}
	}

	// 4. 确保渠道存在（含模型定价）
	channelID, err := s.ensureChannel(ctx, site, groupID, models)
	if err != nil {
		return SyncResult{Error: fmt.Sprintf("ensure channel: %v", err)}
	}

	// 5. 更新幂等标记
	if err := s.siteRepo.UpdateManagedResources(ctx, site.ID, &groupID, &accountID, &channelID); err != nil {
		log.Printf("[UpstreamSync] Warning: failed to update managed resources for site %d: %v", site.ID, err)
	}

	return SyncResult{
		ModelsDiscovered: len(models),
		GroupID:          groupID,
		AccountID:        accountID,
		ChannelID:        channelID,
	}
}

// fetchModels 从上游 GET /v1/models 获取模型列表
func (s *UpstreamSyncService) fetchModels(ctx context.Context, site *UpstreamSite) ([]UpstreamModelInfo, error) {
	url := strings.TrimRight(site.BaseURL, "/") + "/v1/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+site.APIKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("upstream returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []UpstreamModelInfo `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result.Data, nil
}

// ensureGroup 确保分组存在（幂等）
func (s *UpstreamSyncService) ensureGroup(ctx context.Context, site *UpstreamSite) (int64, error) {
	// 如果已有 managed_group_id，先检查是否还存在
	if site.ManagedGroupID != nil {
		g, err := s.groupRepo.GetByIDLite(ctx, *site.ManagedGroupID)
		if err == nil && g != nil {
			// 更新倍率（如果站点倍率改变了）
			if g.RateMultiplier != site.PriceMultiplier {
				g.RateMultiplier = site.PriceMultiplier
				if err := s.groupRepo.Update(ctx, g); err != nil {
					log.Printf("[UpstreamSync] Warning: failed to update group rate_multiplier: %v", err)
				}
			}
			return g.ID, nil
		}
		// 分组已被删除，重新创建
		log.Printf("[UpstreamSync] Managed group %d not found, will recreate", *site.ManagedGroupID)
	}

	// 创建新分组（直接用 repo 以支持 antigravity 平台）
	group := &Group{
		Name:             fmt.Sprintf("上游: %s", site.Name),
		Description:      fmt.Sprintf("自动同步自上游站点 %s (%s)", site.Name, site.BaseURL),
		Platform:         PlatformAntigravity,
		RateMultiplier:   site.PriceMultiplier,
		Status:           StatusActive,
		SubscriptionType: SubscriptionTypeStandard,
	}
	if err := s.groupRepo.Create(ctx, group); err != nil {
		return 0, fmt.Errorf("create group: %w", err)
	}
	return group.ID, nil
}

// ensureAccount 确保账号存在（幂等），并更新 model_mapping
func (s *UpstreamSyncService) ensureAccount(ctx context.Context, site *UpstreamSite, groupID int64, models []UpstreamModelInfo) (int64, error) {
	// 构建 model_mapping：每个上游模型映射到自身
	modelMapping := make(map[string]any, len(models))
	for _, m := range models {
		modelMapping[m.ID] = m.ID
	}

	credentials := map[string]any{
		"api_key":       site.APIKey,
		"base_url":      strings.TrimRight(site.BaseURL, "/"),
		"model_mapping": modelMapping,
	}

	// 如果已有 managed_account_id，更新凭证和 model_mapping
	if site.ManagedAccountID != nil {
		existing, err := s.adminService.GetAccount(ctx, *site.ManagedAccountID)
		if err == nil && existing != nil {
			// 更新凭证（含 model_mapping）
			updateInput := &UpdateAccountInput{
				Credentials: credentials,
			}
			if _, err := s.adminService.UpdateAccount(ctx, existing.ID, updateInput); err != nil {
				log.Printf("[UpstreamSync] Warning: failed to update account credentials: %v", err)
			}
			return existing.ID, nil
		}
		log.Printf("[UpstreamSync] Managed account %d not found, will recreate", *site.ManagedAccountID)
	}

	// 创建新账号
	concurrency := 5
	createInput := &CreateAccountInput{
		Name:                 fmt.Sprintf("上游: %s", site.Name),
		Platform:             PlatformAntigravity,
		Type:                 AccountTypeAPIKey,
		Credentials:          credentials,
		Concurrency:          concurrency,
		Priority:             0,
		GroupIDs:             []int64{groupID},
		SkipMixedChannelCheck: true,
	}
	account, err := s.adminService.CreateAccount(ctx, createInput)
	if err != nil {
		return 0, fmt.Errorf("create account: %w", err)
	}
	return account.ID, nil
}

// ensureChannel 确保渠道存在（幂等），并更新模型定价
func (s *UpstreamSyncService) ensureChannel(ctx context.Context, site *UpstreamSite, groupID int64, models []UpstreamModelInfo) (int64, error) {
	// 构建模型定价列表
	pricingList := s.buildModelPricing(models, site.PriceMultiplier)

	// 如果已有 managed_channel_id，更新定价
	if site.ManagedChannelID != nil {
		existing, err := s.channelService.GetByID(ctx, *site.ManagedChannelID)
		if err == nil && existing != nil {
			updateInput := &UpdateChannelInput{
				ModelPricing:       &pricingList,
				BillingModelSource: BillingModelSourceChannelMapped,
			}
			if _, err := s.channelService.Update(ctx, existing.ID, updateInput); err != nil {
				log.Printf("[UpstreamSync] Warning: failed to update channel pricing: %v", err)
			}
			return existing.ID, nil
		}
		log.Printf("[UpstreamSync] Managed channel %d not found, will recreate", *site.ManagedChannelID)
	}

	// 创建新渠道
	createInput := &CreateChannelInput{
		Name:               fmt.Sprintf("上游: %s", site.Name),
		Description:        fmt.Sprintf("自动同步自上游站点 %s", site.Name),
		GroupIDs:           []int64{groupID},
		ModelPricing:       pricingList,
		BillingModelSource: BillingModelSourceChannelMapped,
		RestrictModels:     false,
	}
	channel, err := s.channelService.Create(ctx, createInput)
	if err != nil {
		return 0, fmt.Errorf("create channel: %w", err)
	}
	return channel.ID, nil
}

// buildModelPricing 根据上游模型列表和倍率构建定价
func (s *UpstreamSyncService) buildModelPricing(models []UpstreamModelInfo, multiplier float64) []ChannelModelPricing {
	// 按平台分组模型
	platformModels := make(map[string][]string)
	for _, m := range models {
		platform := detectPlatform(m.ID)
		platformModels[platform] = append(platformModels[platform], m.ID)
	}

	var pricingList []ChannelModelPricing
	for platform, modelNames := range platformModels {
		// 按定价分组：相同定价的模型合并到一条 pricing
		pricingGroups := groupModelsByPricing(modelNames, multiplier)
		for _, pg := range pricingGroups {
			pricing := ChannelModelPricing{
				Platform:    platform,
				Models:      pg.Models,
				BillingMode: BillingModeToken,
			}
			if pg.InputPrice != nil {
				v := *pg.InputPrice
				pricing.InputPrice = &v
			}
			if pg.OutputPrice != nil {
				v := *pg.OutputPrice
				pricing.OutputPrice = &v
			}
			if pg.CacheWritePrice != nil {
				v := *pg.CacheWritePrice
				pricing.CacheWritePrice = &v
			}
			if pg.CacheReadPrice != nil {
				v := *pg.CacheReadPrice
				pricing.CacheReadPrice = &v
			}
			pricingList = append(pricingList, pricing)
		}
	}
	return pricingList
}

// ── 内置默认价格表（USD per token）──

type modelDefaultPrice struct {
	InputPerToken      float64
	OutputPerToken     float64
	CacheWritePerToken *float64
	CacheReadPerToken  *float64
}

func ptrFloat(v float64) *float64 { return &v }

// defaultModelPrices 内置默认价格（per token, USD）
// 来源: 各官方定价页面，最后更新 2025-01
var defaultModelPrices = map[string]modelDefaultPrice{
	// Claude 系列 (Anthropic)
	"claude-opus-4-6-thinking":   {InputPerToken: 15e-6, OutputPerToken: 75e-6, CacheWritePerToken: ptrFloat(18.75e-6), CacheReadPerToken: ptrFloat(1.50e-6)},
	"claude-opus-4-5-thinking":   {InputPerToken: 15e-6, OutputPerToken: 75e-6, CacheWritePerToken: ptrFloat(18.75e-6), CacheReadPerToken: ptrFloat(1.50e-6)},
	"claude-sonnet-4-6":          {InputPerToken: 3e-6, OutputPerToken: 15e-6, CacheWritePerToken: ptrFloat(3.75e-6), CacheReadPerToken: ptrFloat(0.30e-6)},
	"claude-sonnet-4-5":          {InputPerToken: 3e-6, OutputPerToken: 15e-6, CacheWritePerToken: ptrFloat(3.75e-6), CacheReadPerToken: ptrFloat(0.30e-6)},
	"claude-sonnet-4-5-thinking": {InputPerToken: 3e-6, OutputPerToken: 15e-6, CacheWritePerToken: ptrFloat(3.75e-6), CacheReadPerToken: ptrFloat(0.30e-6)},
	"claude-haiku-4-5":           {InputPerToken: 0.80e-6, OutputPerToken: 4e-6, CacheWritePerToken: ptrFloat(1e-6), CacheReadPerToken: ptrFloat(0.08e-6)},

	// GPT 系列 (OpenAI)
	"gpt-4o":            {InputPerToken: 2.5e-6, OutputPerToken: 10e-6},
	"gpt-4o-mini":       {InputPerToken: 0.15e-6, OutputPerToken: 0.60e-6},
	"gpt-4-turbo":       {InputPerToken: 10e-6, OutputPerToken: 30e-6},
	"gpt-5.4":           {InputPerToken: 2.5e-6, OutputPerToken: 15e-6},
	"gpt-5.4-mini":      {InputPerToken: 0.75e-6, OutputPerToken: 4.5e-6},
	"o1":                {InputPerToken: 15e-6, OutputPerToken: 60e-6},
	"o1-mini":           {InputPerToken: 3e-6, OutputPerToken: 12e-6},
	"o3":                {InputPerToken: 10e-6, OutputPerToken: 40e-6},
	"o3-mini":           {InputPerToken: 1.1e-6, OutputPerToken: 4.4e-6},
	"o4-mini":           {InputPerToken: 1.1e-6, OutputPerToken: 4.4e-6},
	"gpt-oss-120b-medium": {InputPerToken: 2e-6, OutputPerToken: 8e-6},

	// Gemini 系列 (Google)
	"gemini-2.5-pro":      {InputPerToken: 1.25e-6, OutputPerToken: 10e-6},
	"gemini-2.5-flash":    {InputPerToken: 0.15e-6, OutputPerToken: 0.60e-6},
	"gemini-3-flash":      {InputPerToken: 0.15e-6, OutputPerToken: 0.60e-6},
	"gemini-3-pro-high":   {InputPerToken: 1.25e-6, OutputPerToken: 10e-6},
	"gemini-3-pro-low":    {InputPerToken: 0.625e-6, OutputPerToken: 5e-6},
	"gemini-3.1-pro-high": {InputPerToken: 1.25e-6, OutputPerToken: 10e-6},
	"gemini-3.1-pro-low":  {InputPerToken: 0.625e-6, OutputPerToken: 5e-6},
}

// pricingGroup 用于定价分组
type pricingGroup struct {
	Models          []string
	InputPrice      *float64
	OutputPrice     *float64
	CacheWritePrice *float64
	CacheReadPrice  *float64
}

// groupModelsByPricing 将模型按定价分组
func groupModelsByPricing(modelNames []string, multiplier float64) []pricingGroup {
	type priceKey struct {
		input, output, cacheWrite, cacheRead float64
		hasInput, hasOutput                  bool
		hasCacheWrite, hasCacheRead          bool
	}

	groups := make(map[priceKey]*pricingGroup)
	var unknownModels []string

	for _, name := range modelNames {
		price, known := lookupModelPrice(name)
		if !known {
			unknownModels = append(unknownModels, name)
			continue
		}

		key := priceKey{
			input:         price.InputPerToken * multiplier,
			output:        price.OutputPerToken * multiplier,
			hasInput:      true,
			hasOutput:     true,
			hasCacheWrite: price.CacheWritePerToken != nil,
			hasCacheRead:  price.CacheReadPerToken != nil,
		}
		if price.CacheWritePerToken != nil {
			key.cacheWrite = *price.CacheWritePerToken * multiplier
		}
		if price.CacheReadPerToken != nil {
			key.cacheRead = *price.CacheReadPerToken * multiplier
		}

		if pg, ok := groups[key]; ok {
			pg.Models = append(pg.Models, name)
		} else {
			inputP := price.InputPerToken * multiplier
			outputP := price.OutputPerToken * multiplier
			pg := &pricingGroup{
				Models:     []string{name},
				InputPrice: &inputP,
				OutputPrice: &outputP,
			}
			if price.CacheWritePerToken != nil {
				v := *price.CacheWritePerToken * multiplier
				pg.CacheWritePrice = &v
			}
			if price.CacheReadPerToken != nil {
				v := *price.CacheReadPerToken * multiplier
				pg.CacheReadPrice = &v
			}
			groups[key] = pg
		}
	}

	var result []pricingGroup
	for _, pg := range groups {
		result = append(result, *pg)
	}

	// 未知模型：不设价格，让管理员手动配置
	if len(unknownModels) > 0 {
		result = append(result, pricingGroup{
			Models: unknownModels,
		})
	}
	return result
}

// lookupModelPrice 查找模型默认价格（精确匹配 → 前缀匹配）
func lookupModelPrice(model string) (modelDefaultPrice, bool) {
	// 精确匹配
	if p, ok := defaultModelPrices[model]; ok {
		return p, true
	}
	// 前缀匹配（处理带版本号的模型名，如 claude-sonnet-4-5-20250929）
	bestMatch := ""
	for key := range defaultModelPrices {
		if strings.HasPrefix(model, key) && len(key) > len(bestMatch) {
			bestMatch = key
		}
	}
	if bestMatch != "" {
		return defaultModelPrices[bestMatch], true
	}
	return modelDefaultPrice{}, false
}

// detectPlatform 根据模型名推断平台
func detectPlatform(modelID string) string {
	lower := strings.ToLower(modelID)
	switch {
	case strings.HasPrefix(lower, "claude"):
		return PlatformAnthropic
	case strings.HasPrefix(lower, "gpt"), strings.HasPrefix(lower, "o1"), strings.HasPrefix(lower, "o3"),
		strings.HasPrefix(lower, "o4"), strings.HasPrefix(lower, "chatgpt"):
		return PlatformOpenAI
	case strings.HasPrefix(lower, "gemini"), strings.HasPrefix(lower, "tab_"):
		return PlatformGemini
	default:
		return PlatformAntigravity
	}
}
