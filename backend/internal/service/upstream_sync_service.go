package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// UpstreamSyncService 上游站点同步服务
type UpstreamSyncService struct {
	siteRepo       UpstreamSiteRepository
	resourceRepo   UpstreamManagedResourceRepository
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
	resourceRepo UpstreamManagedResourceRepository,
	groupRepo GroupRepository,
	adminService AdminService,
	channelService *ChannelService,
) *UpstreamSyncService {
	return &UpstreamSyncService{
		siteRepo:       siteRepo,
		resourceRepo:   resourceRepo,
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
			log.Printf("[UpstreamSync] Site %q (#%d) sync success: %d models, %d keys",
				site.Name, site.ID, result.ModelsDiscovered, result.KeysDiscovered)
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

	if site.CredentialMode == "login" {
		// login 模式：先发现所有 key，然后用第一个 key 获取模型
		accessToken, err := s.getAccessToken(ctx, site)
		if err != nil {
			return nil, fmt.Errorf("get access token: %w", err)
		}
		keys, err := s.discoverKeys(ctx, site, accessToken)
		if err != nil {
			return nil, fmt.Errorf("discover keys: %w", err)
		}
		if len(keys) == 0 {
			return nil, fmt.Errorf("no API keys found for this account")
		}
		return s.fetchModelsWithKey(ctx, site, keys[0].Key)
	}

	return s.fetchModelsWithKey(ctx, site, site.APIKey)
}

// CheckUpstreamBalance 查询上游余额
func (s *UpstreamSyncService) CheckUpstreamBalance(ctx context.Context, siteID int64) (*UpstreamBalanceInfo, error) {
	site, err := s.siteRepo.GetByID(ctx, siteID)
	if err != nil {
		return nil, err
	}

	if site.CredentialMode == "login" {
		// login 模式: 使用 JWT 查 /api/v1/auth/me 获取 balance
		accessToken, err := s.getAccessToken(ctx, site)
		if err != nil {
			return nil, fmt.Errorf("get access token: %w", err)
		}
		return s.fetchBalanceViaJWT(ctx, site, accessToken)
	}

	// api_key 模式: /v1/usage
	return s.fetchBalanceViaAPIKey(ctx, site)
}

// DeleteSiteWithResources 删除上游站点及其所有本地资源（分组/账号/渠道）
func (s *UpstreamSyncService) DeleteSiteWithResources(ctx context.Context, siteID int64) error {
	// 1. 列出该站点所有 managed resources
	resources, err := s.resourceRepo.ListBySiteID(ctx, siteID)
	if err != nil {
		return fmt.Errorf("list managed resources: %w", err)
	}

	// 2. 逐个删除本地资源（渠道 → 账号 → 分组 顺序，避免外键冲突）
	for _, res := range resources {
		if res.ManagedChannelID != nil {
			if err := s.channelService.Delete(ctx, *res.ManagedChannelID); err != nil {
				log.Printf("[UpstreamSync] Warning: failed to delete channel %d: %v", *res.ManagedChannelID, err)
			}
		}
		if res.ManagedAccountID != nil {
			if err := s.adminService.DeleteAccount(ctx, *res.ManagedAccountID); err != nil {
				log.Printf("[UpstreamSync] Warning: failed to delete account %d: %v", *res.ManagedAccountID, err)
			}
		}
		if res.ManagedGroupID != nil {
			if err := s.adminService.DeleteGroup(ctx, *res.ManagedGroupID); err != nil {
				log.Printf("[UpstreamSync] Warning: failed to delete group %d: %v", *res.ManagedGroupID, err)
			}
		}
	}

	// 3. 删除 managed resources 记录
	if err := s.resourceRepo.DeleteBySiteID(ctx, siteID); err != nil {
		log.Printf("[UpstreamSync] Warning: failed to delete managed resources for site %d: %v", siteID, err)
	}

	// 4. 删除站点
	return s.siteRepo.Delete(ctx, siteID)
}

// ListManagedResources 列出站点的托管资源
func (s *UpstreamSyncService) ListManagedResources(ctx context.Context, siteID int64) ([]*UpstreamManagedResource, error) {
	return s.resourceRepo.ListBySiteID(ctx, siteID)
}

// SetSiteResourcesStatus 批量设置站点下所有托管资源及其关联本地资源的状态
func (s *UpstreamSyncService) SetSiteResourcesStatus(ctx context.Context, siteID int64, status string) {
	resources, err := s.resourceRepo.ListBySiteID(ctx, siteID)
	if err != nil {
		log.Printf("[UpstreamSync] Warning: failed to list resources for site %d: %v", siteID, err)
		return
	}
	for _, res := range resources {
		if res.Status == status {
			continue
		}
		disabledBy := ""
		if status == "disabled" {
			disabledBy = "manual"
		}
		if err := s.resourceRepo.UpdateStatus(ctx, res.ID, status); err != nil {
			log.Printf("[UpstreamSync] Warning: failed to update resource %d status: %v", res.ID, err)
			continue
		}
		_ = s.resourceRepo.UpdateDisabledBy(ctx, res.ID, disabledBy)
		s.setLocalResourceStatus(ctx, res, status)
	}
}

// DeleteResource 删除单个托管资源及其关联的本地 channel/account/group
func (s *UpstreamSyncService) DeleteResource(ctx context.Context, resourceID int64) error {
	res, err := s.resourceRepo.GetByID(ctx, resourceID)
	if err != nil || res == nil {
		return fmt.Errorf("resource not found")
	}

	if res.ManagedChannelID != nil {
		if err := s.channelService.Delete(ctx, *res.ManagedChannelID); err != nil {
			log.Printf("[UpstreamSync] Warning: failed to delete channel %d: %v", *res.ManagedChannelID, err)
		}
	}
	if res.ManagedAccountID != nil {
		if err := s.adminService.DeleteAccount(ctx, *res.ManagedAccountID); err != nil {
			log.Printf("[UpstreamSync] Warning: failed to delete account %d: %v", *res.ManagedAccountID, err)
		}
	}
	if res.ManagedGroupID != nil {
		if err := s.adminService.DeleteGroup(ctx, *res.ManagedGroupID); err != nil {
			log.Printf("[UpstreamSync] Warning: failed to delete group %d: %v", *res.ManagedGroupID, err)
		}
	}

	return s.resourceRepo.DeleteByID(ctx, resourceID)
}

// UpdateResourceMultiplier 更新单个托管资源的倍率，并同步更新本地分组的 rate_multiplier
func (s *UpstreamSyncService) UpdateResourceMultiplier(ctx context.Context, resourceID int64, multiplier float64) (*UpstreamManagedResource, error) {
	if err := s.resourceRepo.UpdatePriceMultiplier(ctx, resourceID, multiplier); err != nil {
		return nil, err
	}

	// 同步更新本地分组的 rate_multiplier
	res, err := s.resourceRepo.GetByID(ctx, resourceID)
	if err != nil {
		return nil, err
	}
	if res != nil && res.ManagedGroupID != nil {
		// multiplier 现在是加价百分比，0 表示使用站点默认
		markupPercent := multiplier
		if markupPercent == 0 {
			// 使用站点默认加价百分比
			site, err := s.siteRepo.GetByID(ctx, res.UpstreamSiteID)
			if err == nil && site != nil {
				markupPercent = site.PriceMultiplier
			}
		}
		// 本地倍率 = 上游倍率 × (1 + 加价百分比/100)
		upstreamRate := res.UpstreamRateMultiplier
		if upstreamRate <= 0 {
			upstreamRate = 1.0
		}
		effectiveMultiplier := upstreamRate * (1 + markupPercent/100)
		if effectiveMultiplier <= 0 {
			effectiveMultiplier = upstreamRate
		}
		g, err := s.groupRepo.GetByIDLite(ctx, *res.ManagedGroupID)
		if err == nil && g != nil && g.RateMultiplier != effectiveMultiplier {
			g.RateMultiplier = effectiveMultiplier
			if err := s.groupRepo.Update(ctx, g); err != nil {
				log.Printf("[UpstreamSync] Warning: failed to update group rate_multiplier: %v", err)
			}
		}
	}
	return res, nil
}

// UpdateResourceModelFilter 更新托管资源的模型过滤规则
func (s *UpstreamSyncService) UpdateResourceModelFilter(ctx context.Context, resourceID int64, modelFilter string) error {
	return s.resourceRepo.UpdateModelFilter(ctx, resourceID, modelFilter)
}

// ToggleResource 切换资源状态 (active ↔ disabled)，同步更新本地分组/账号/渠道状态
func (s *UpstreamSyncService) ToggleResource(ctx context.Context, resourceID int64) (*UpstreamManagedResource, error) {
	res, err := s.resourceRepo.GetByID(ctx, resourceID)
	if err != nil || res == nil {
		return nil, fmt.Errorf("resource not found")
	}
	newStatus := "disabled"
	disabledBy := "manual"
	if res.Status == "disabled" {
		newStatus = "active"
		disabledBy = ""
	}
	if err := s.resourceRepo.UpdateStatus(ctx, resourceID, newStatus); err != nil {
		return nil, err
	}
	if err := s.resourceRepo.UpdateDisabledBy(ctx, resourceID, disabledBy); err != nil {
		log.Printf("[UpstreamSync] Warning: failed to update disabled_by for resource %d: %v", resourceID, err)
	}

	s.setLocalResourceStatus(ctx, res, newStatus)

	return s.resourceRepo.GetByID(ctx, resourceID)
}

// setLocalResourceStatus 设置资源关联的本地 group/account/channel 状态
func (s *UpstreamSyncService) setLocalResourceStatus(ctx context.Context, res *UpstreamManagedResource, status string) {
	if res.ManagedGroupID != nil {
		g, err := s.groupRepo.GetByIDLite(ctx, *res.ManagedGroupID)
		if err == nil && g != nil && g.Status != status {
			g.Status = status
			if err := s.groupRepo.Update(ctx, g); err != nil {
				log.Printf("[UpstreamSync] Warning: failed to update group %d status to %s: %v", *res.ManagedGroupID, status, err)
			}
		}
	}
	if res.ManagedAccountID != nil {
		if _, err := s.adminService.UpdateAccount(ctx, *res.ManagedAccountID, &UpdateAccountInput{
			Status: status,
		}); err != nil {
			log.Printf("[UpstreamSync] Warning: failed to update account %d status to %s: %v", *res.ManagedAccountID, status, err)
		}
	}
	if res.ManagedChannelID != nil {
		if _, err := s.channelService.Update(ctx, *res.ManagedChannelID, &UpdateChannelInput{
			Status: status,
		}); err != nil {
			log.Printf("[UpstreamSync] Warning: failed to update channel %d status to %s: %v", *res.ManagedChannelID, status, err)
		}
	}
}

// ── 核心同步逻辑 ──

func (s *UpstreamSyncService) syncSite(ctx context.Context, site *UpstreamSite) SyncResult {
	if site.CredentialMode == "login" {
		return s.syncSiteLoginMode(ctx, site)
	}
	return s.syncSiteAPIKeyMode(ctx, site)
}

// syncSiteAPIKeyMode api_key 模式同步（单 Key）
func (s *UpstreamSyncService) syncSiteAPIKeyMode(ctx context.Context, site *UpstreamSite) SyncResult {
	// 1. 获取上游模型列表
	models, err := s.fetchModelsWithKey(ctx, site, site.APIKey)
	if err != nil {
		return SyncResult{Error: fmt.Sprintf("fetch models: %v", err)}
	}
	if len(models) == 0 {
		return SyncResult{Error: "upstream returned 0 models"}
	}

	// 2. Upsert 到 managed resource 表
	keyID := "apikey-" + strconv.FormatInt(site.ID, 10)
	res := &UpstreamManagedResource{
		UpstreamSiteID:  site.ID,
		UpstreamKeyID:   keyID,
		UpstreamKeyPrefix: maskAPIKey(site.APIKey),
		UpstreamKeyName: "Manual API Key",
		APIKey:          site.APIKey,
		Status:          "active",
	}
	if err := s.resourceRepo.Upsert(ctx, res); err != nil {
		return SyncResult{Error: fmt.Sprintf("upsert resource: %v", err)}
	}

	// Apply model filter if configured (re-read from DB to get persisted model_filter)
	existing, _ := s.resourceRepo.GetBySiteAndKeyID(ctx, site.ID, res.UpstreamKeyID)
	if existing != nil && existing.ModelFilter != "" {
		res.ModelFilter = existing.ModelFilter
		models = res.FilterModels(models)
		if len(models) == 0 {
			log.Printf("[UpstreamSync] Site %q: model_filter %q filtered out all %d models", site.Name, existing.ModelFilter, len(models))
		}
	}

	// 3. 确保分组/账号/渠道存在
	// api_key 模式没有上游倍率信息，基准倍率 = 1.0
	effectiveMultiplier := 1.0 * (1 + site.PriceMultiplier/100)
	if effectiveMultiplier <= 0 {
		effectiveMultiplier = 1.0
	}
	groupID, accountID, channelID, err := s.ensureLocalResources(ctx, site, res, models, effectiveMultiplier)
	if err != nil {
		return SyncResult{Error: err.Error()}
	}

	// 4. 更新 managed resource 的关联 ID 和模型数
	_ = s.resourceRepo.UpdateManagedIDs(ctx, res.ID, &groupID, &accountID, &channelID)
	_ = s.resourceRepo.UpdateModelCount(ctx, res.ID, len(models))

	return SyncResult{
		ModelsDiscovered: len(models),
		KeysDiscovered:   1,
		GroupID:          groupID,
		AccountID:        accountID,
		ChannelID:        channelID,
	}
}

// syncSiteLoginMode 邮箱密码登录模式同步（多 Key 自动发现）
func (s *UpstreamSyncService) syncSiteLoginMode(ctx context.Context, site *UpstreamSite) SyncResult {
	log.Printf("[UpstreamSync] Site %q (#%d): starting login-mode sync (email=%q, hasPassword=%v)",
		site.Name, site.ID, site.Email, site.Password != "")

	// 1. 获取 access token
	accessToken, err := s.getAccessToken(ctx, site)
	if err != nil {
		return SyncResult{Error: fmt.Sprintf("login failed: %v", err)}
	}
	log.Printf("[UpstreamSync] Site %q (#%d): got access token (len=%d)", site.Name, site.ID, len(accessToken))

	// 2. 发现所有 API Key
	keys, err := s.discoverKeys(ctx, site, accessToken)
	if err != nil {
		return SyncResult{Error: fmt.Sprintf("discover keys: %v", err)}
	}
	if len(keys) == 0 {
		return SyncResult{Error: "no API keys found for this account"}
	}

	log.Printf("[UpstreamSync] Site %q: discovered %d API key(s)", site.Name, len(keys))

	// 3. 获取上游分组元信息（名称+倍率）
	groupMeta := s.fetchUpstreamGroupMeta(ctx, site, accessToken)

	// 4. 对每个 key 获取模型并创建本地资源
	totalModels := 0
	var activeKeyIDs []string
	for _, key := range keys {
		keyID := strconv.FormatInt(key.ID, 10)

		// 获取该 key 下的模型
		models, err := s.fetchModelsWithKey(ctx, site, key.Key)
		if err != nil {
			log.Printf("[UpstreamSync] Site %q key %s: fetch models failed: %v", site.Name, key.Name, err)
			// Key exists but can't fetch models (e.g. not assigned to group) —
			// don't add to activeKeyIDs so DisableStale can auto-disable it
			continue
		}

		// Only mark key as active after successful model fetch
		activeKeyIDs = append(activeKeyIDs, keyID)

		// 确定显示名称：优先使用上游分组名称，否则用 key name
		displayName := key.Name
		if key.GroupID != nil {
			if gm, ok := groupMeta[*key.GroupID]; ok && gm.Name != "" {
				displayName = gm.Name
			}
		}

		// Upsert managed resource
		res := &UpstreamManagedResource{
			UpstreamSiteID:  site.ID,
			UpstreamKeyID:   keyID,
			UpstreamKeyPrefix: maskAPIKey(key.Key),
			UpstreamKeyName: displayName,
			APIKey:          key.Key,
			Status:          "active",
		}
		if key.GroupID != nil {
			res.UpstreamGroupID = key.GroupID
			// 记录上游分组倍率和描述
			if gm, ok := groupMeta[*key.GroupID]; ok {
				res.UpstreamRateMultiplier = gm.RateMultiplier
				res.UpstreamGroupDescription = gm.Description
			}
		}
		if err := s.resourceRepo.Upsert(ctx, res); err != nil {
			log.Printf("[UpstreamSync] Site %q key %s: upsert resource failed: %v", site.Name, key.Name, err)
			continue
		}

		// 更新上游倍率
		if res.UpstreamRateMultiplier > 0 {
			_ = s.resourceRepo.UpdateUpstreamRateMultiplier(ctx, res.ID, res.UpstreamRateMultiplier)
		}

		// 从已有记录获取资源状态和自定义倍率
		existing, _ := s.resourceRepo.GetBySiteAndKeyID(ctx, site.ID, res.UpstreamKeyID)
		if existing != nil {
			res.PriceMultiplier = existing.PriceMultiplier
			if existing.Status == "disabled" {
				if existing.DisabledBy == "auto" {
					// 上游重新上架，自动恢复
					_ = s.resourceRepo.UpdateStatus(ctx, existing.ID, "active")
					_ = s.resourceRepo.UpdateDisabledBy(ctx, existing.ID, "")
					s.setLocalResourceStatus(ctx, existing, "active")
					log.Printf("[UpstreamSync] Site %q key %s: auto-re-enabled (upstream restored)", site.Name, displayName)
				} else {
					// 手动禁用，跳过
					log.Printf("[UpstreamSync] Site %q key %s: manually disabled, skipping", site.Name, displayName)
					_ = s.resourceRepo.UpdateModelCount(ctx, res.ID, len(models))
					totalModels += len(models)
					continue
				}
			}
		}

		// Apply model filter if configured
		if existing != nil && existing.ModelFilter != "" {
			res.ModelFilter = existing.ModelFilter
			models = res.FilterModels(models)
		}

		// 确定加价百分比：资源自定义 > 站点默认
		markupPercent := site.PriceMultiplier // 语义：加价百分比（如 30 = 加价 30%）
		if res.PriceMultiplier > 0 {
			markupPercent = res.PriceMultiplier
		}
		// 本地倍率 = 上游倍率 × (1 + 加价百分比/100)
		upstreamRate := res.UpstreamRateMultiplier
		if upstreamRate <= 0 {
			upstreamRate = 1.0
		}
		effectiveMultiplier := upstreamRate * (1 + markupPercent/100)
		if effectiveMultiplier <= 0 {
			effectiveMultiplier = upstreamRate
		}

		// 确保本地 group/account/channel
		groupID, accountID, channelID, err := s.ensureLocalResources(ctx, site, res, models, effectiveMultiplier)
		if err != nil {
			log.Printf("[UpstreamSync] Site %q key %s: ensure resources failed: %v", site.Name, key.Name, err)
			continue
		}

		// 更新关联 ID 和模型数
		_ = s.resourceRepo.UpdateManagedIDs(ctx, res.ID, &groupID, &accountID, &channelID)
		_ = s.resourceRepo.UpdateModelCount(ctx, res.ID, len(models))

		totalModels += len(models)
	}

	// 5. 自动禁用上游已下架的 key（软禁用，不删除，上架后自动恢复）
	staleResources, err := s.resourceRepo.DisableStale(ctx, site.ID, activeKeyIDs)
	if err != nil {
		log.Printf("[UpstreamSync] Site %q: disable stale resources failed: %v", site.Name, err)
	} else if len(staleResources) > 0 {
		for _, res := range staleResources {
			s.setLocalResourceStatus(ctx, res, "disabled")
			log.Printf("[UpstreamSync] Site %q: auto-disabled resource %q (upstream removed)", site.Name, res.UpstreamKeyName)
		}
	}

	return SyncResult{
		ModelsDiscovered: totalModels,
		KeysDiscovered:   len(keys),
	}
}

// ── 登录与 Token 管理 ──

// upstreamKeyInfo 上游 API Key 信息（来自 /api/v1/keys）
type upstreamKeyInfo struct {
	ID      int64  `json:"id"`
	Key     string `json:"key"`
	Name    string `json:"name"`
	GroupID *int64 `json:"group_id"`
	Status  string `json:"status"`
}

// upstreamGroupInfo 上游分组信息（来自 /api/v1/groups/available）
type upstreamGroupInfo struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	RateMultiplier float64 `json:"rate_multiplier"`
}

// upstreamGroupMeta 分组元信息（名称+描述+倍率）
type upstreamGroupMeta struct {
	Name           string
	Description    string
	RateMultiplier float64
}

// fetchUpstreamGroupMeta 获取上游分组元信息映射 (groupID → {Name, RateMultiplier})
func (s *UpstreamSyncService) fetchUpstreamGroupMeta(ctx context.Context, site *UpstreamSite, accessToken string) map[int64]upstreamGroupMeta {
	url := strings.TrimRight(site.BaseURL, "/") + "/api/v1/groups/available"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Printf("[UpstreamSync] Site %q: create groups request failed: %v", site.Name, err)
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("[UpstreamSync] Site %q: fetch groups failed: %v", site.Name, err)
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		log.Printf("[UpstreamSync] Site %q: groups endpoint returned %d: %s", site.Name, resp.StatusCode, string(body))
		return nil
	}

	var result struct {
		Code int                 `json:"code"`
		Data []upstreamGroupInfo `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[UpstreamSync] Site %q: decode groups response: %v", site.Name, err)
		return nil
	}

	meta := make(map[int64]upstreamGroupMeta, len(result.Data))
	for _, g := range result.Data {
		meta[g.ID] = upstreamGroupMeta{Name: g.Name, Description: g.Description, RateMultiplier: g.RateMultiplier}
	}
	log.Printf("[UpstreamSync] Site %q: fetched %d upstream groups", site.Name, len(meta))
	return meta
}

// getAccessToken 获取上游的 JWT access token（优先缓存 → 刷新 → 登录）
func (s *UpstreamSyncService) getAccessToken(ctx context.Context, site *UpstreamSite) (string, error) {
	// 1. 检查缓存 token 是否有效（至少还有 5 分钟有效期）
	if site.CachedAccessToken != "" && site.TokenExpiresAt != nil {
		if time.Until(*site.TokenExpiresAt) > 5*time.Minute {
			return site.CachedAccessToken, nil
		}
	}

	// 2. 尝试 refresh token
	if site.CachedRefreshToken != "" {
		accessToken, refreshToken, expiresIn, err := s.refreshToken(ctx, site)
		if err == nil {
			expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
			_ = s.siteRepo.UpdateTokenCache(ctx, site.ID, accessToken, refreshToken, &expiresAt)
			site.CachedAccessToken = accessToken
			site.CachedRefreshToken = refreshToken
			site.TokenExpiresAt = &expiresAt
			return accessToken, nil
		}
		log.Printf("[UpstreamSync] Site %q: refresh token failed (%v), falling back to login", site.Name, err)
	}

	// 3. 全量登录
	accessToken, refreshToken, expiresIn, err := s.loginUpstream(ctx, site)
	if err != nil {
		_ = s.siteRepo.ClearTokenCache(ctx, site.ID)
		return "", err
	}

	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
	_ = s.siteRepo.UpdateTokenCache(ctx, site.ID, accessToken, refreshToken, &expiresAt)
	site.CachedAccessToken = accessToken
	site.CachedRefreshToken = refreshToken
	site.TokenExpiresAt = &expiresAt
	return accessToken, nil
}

// loginUpstream 使用邮箱+密码登录上游
func (s *UpstreamSyncService) loginUpstream(ctx context.Context, site *UpstreamSite) (accessToken, refreshToken string, expiresIn int, err error) {
	url := strings.TrimRight(site.BaseURL, "/") + "/api/v1/auth/login"

	body, _ := json.Marshal(map[string]string{
		"email":           site.Email,
		"password":        site.Password,
		"turnstile_token": "",
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", "", 0, fmt.Errorf("create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", "", 0, fmt.Errorf("login request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))

	if resp.StatusCode != http.StatusOK {
		return "", "", 0, fmt.Errorf("login returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int    `json:"expires_in"`
			Requires2FA  bool   `json:"requires_2fa"`
			TempToken    string `json:"temp_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", "", 0, fmt.Errorf("decode login response: %w", err)
	}

	if result.Data.Requires2FA {
		return "", "", 0, fmt.Errorf("upstream requires 2FA authentication, which is not supported for server-to-server login")
	}

	if result.Data.AccessToken == "" {
		return "", "", 0, fmt.Errorf("login failed: %s", result.Message)
	}

	return result.Data.AccessToken, result.Data.RefreshToken, result.Data.ExpiresIn, nil
}

// refreshToken 使用 refresh token 刷新 access token
func (s *UpstreamSyncService) refreshToken(ctx context.Context, site *UpstreamSite) (accessToken, refreshToken string, expiresIn int, err error) {
	url := strings.TrimRight(site.BaseURL, "/") + "/api/v1/auth/refresh"

	body, _ := json.Marshal(map[string]string{
		"refresh_token": site.CachedRefreshToken,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", "", 0, fmt.Errorf("create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", "", 0, fmt.Errorf("refresh request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", "", 0, fmt.Errorf("refresh returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Code int `json:"code"`
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int    `json:"expires_in"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", 0, fmt.Errorf("decode refresh response: %w", err)
	}

	if result.Data.AccessToken == "" {
		return "", "", 0, fmt.Errorf("refresh failed: empty access token")
	}

	return result.Data.AccessToken, result.Data.RefreshToken, result.Data.ExpiresIn, nil
}

// discoverKeys 使用 JWT 发现上游所有 API Key
func (s *UpstreamSyncService) discoverKeys(ctx context.Context, site *UpstreamSite, accessToken string) ([]upstreamKeyInfo, error) {
	url := strings.TrimRight(site.BaseURL, "/") + "/api/v1/keys?page_size=100"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create keys request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("keys request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("keys endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Code int `json:"code"`
		Data struct {
			Items []upstreamKeyInfo `json:"items"`
			Total int               `json:"total"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode keys response: %w", err)
	}

	// 过滤 active 的 key
	var activeKeys []upstreamKeyInfo
	for _, k := range result.Data.Items {
		if k.Status == "active" {
			activeKeys = append(activeKeys, k)
		}
	}
	return activeKeys, nil
}

// ── 本地资源管理 ──

// ensureLocalResources 确保本地 group/account/channel 存在（幂等）
func (s *UpstreamSyncService) ensureLocalResources(
	ctx context.Context,
	site *UpstreamSite,
	res *UpstreamManagedResource,
	models []UpstreamModelInfo,
	effectiveMultiplier float64,
) (groupID, accountID, channelID int64, err error) {
	// 从已有的 managed resource 获取之前创建的 ID
	existing, _ := s.resourceRepo.GetBySiteAndKeyID(ctx, site.ID, res.UpstreamKeyID)

	// 1. 确保分组
	groupID, err = s.ensureGroup(ctx, site, res, existing, effectiveMultiplier, models)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("ensure group: %v", err)
	}

	// 2. 确保账号
	accountID, err = s.ensureAccount(ctx, site, res, existing, groupID, models)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("ensure account: %v", err)
	}

	// 3. 确保渠道
	channelID, err = s.ensureChannel(ctx, site, res, existing, groupID, models, effectiveMultiplier)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("ensure channel: %v", err)
	}

	return groupID, accountID, channelID, nil
}

// ensureGroup 确保分组存在（幂等）
func (s *UpstreamSyncService) ensureGroup(ctx context.Context, site *UpstreamSite, res *UpstreamManagedResource, existing *UpstreamManagedResource, effectiveMultiplier float64, models []UpstreamModelInfo) (int64, error) {
	// 计算期望的名称
	groupName := fmt.Sprintf("上游: %s", site.Name)
	if res.UpstreamKeyName != "" && res.UpstreamKeyName != "Manual API Key" {
		groupName = fmt.Sprintf("%s (%s)", res.UpstreamKeyName, site.Name)
	}
	groupDesc := ""
	if res.UpstreamGroupDescription != "" {
		groupDesc = res.UpstreamGroupDescription
	}
	// 剥离上游描述中的价格信息（如 "价格 1k:0.06 2k:0.12 4k:0.24"）
	groupDesc = stripUpstreamPriceText(groupDesc)

	// 检测是否为纯图片模型分组
	// 仅当本地分组没有 ImagePrice 配置时，才在描述中嵌入按次计费标记
	if len(models) > 0 {
		allImage := true
		var maxPrice float64
		for _, m := range models {
			if !IsImageModel(m.ID) {
				allImage = false
				break
			}
			if p, ok := LookupImageModelPrice(m.ID); ok && p > maxPrice {
				maxPrice = p
			}
		}
		// 检查本地分组是否已有 ImagePrice 配置
		hasLocalImagePrice := false
		if existing != nil && existing.ManagedGroupID != nil {
			if g, err := s.groupRepo.GetByIDLite(ctx, *existing.ManagedGroupID); err == nil && g != nil {
				hasLocalImagePrice = g.ImagePrice1K != nil && *g.ImagePrice1K > 0
			}
		}
		if allImage && maxPrice > 0 && !hasLocalImagePrice {
			priceTag := fmt.Sprintf("[per_request:%.4f]", maxPrice*effectiveMultiplier)
			// 移除旧标记（如果有）再追加
			if idx := strings.Index(groupDesc, "[per_request:"); idx >= 0 {
				end := strings.Index(groupDesc[idx:], "]")
				if end >= 0 {
					groupDesc = strings.TrimSpace(groupDesc[:idx] + groupDesc[idx+end+1:])
				}
			}
			if groupDesc != "" {
				groupDesc = groupDesc + " " + priceTag
			} else {
				groupDesc = priceTag
			}
		} else if hasLocalImagePrice {
			// 有本地 ImagePrice 配置时，移除描述中已有的 [per_request:] 标记
			if idx := strings.Index(groupDesc, "[per_request:"); idx >= 0 {
				end := strings.Index(groupDesc[idx:], "]")
				if end >= 0 {
					groupDesc = strings.TrimSpace(groupDesc[:idx] + groupDesc[idx+end+1:])
				}
			}
		}
	}

	// 如果已有 managed_group_id，先检查是否还存在
	if existing != nil && existing.ManagedGroupID != nil {
		g, err := s.groupRepo.GetByIDLite(ctx, *existing.ManagedGroupID)
		if err == nil && g != nil {
			needUpdate := false
			if g.RateMultiplier != effectiveMultiplier {
				g.RateMultiplier = effectiveMultiplier
				needUpdate = true
			}
			// Don't overwrite group name if user has customized it
			if g.Name == "" {
				g.Name = groupName
				needUpdate = true
			}
			if groupDesc != "" && g.Description != groupDesc {
				g.Description = groupDesc
				needUpdate = true
			}
			if needUpdate {
				if err := s.groupRepo.Update(ctx, g); err != nil {
					log.Printf("[UpstreamSync] Warning: failed to update group: %v", err)
				}
			}
			return g.ID, nil
		}
		log.Printf("[UpstreamSync] Managed group %d not found, will recreate", *existing.ManagedGroupID)
	}

	// 创建新分组 — 使用上游分组名称
	group := &Group{
		Name:             groupName,
		Description:      groupDesc,
		Platform:         PlatformAntigravity,
		RateMultiplier:   effectiveMultiplier,
		Status:           StatusActive,
		SubscriptionType: SubscriptionTypeStandard,
	}
	if err := s.groupRepo.Create(ctx, group); err != nil {
		if strings.Contains(err.Error(), "GROUP_EXISTS") || strings.Contains(err.Error(), "unique") {
			// 同名分组已存在，先尝试查找并复用
			if existing, lookupErr := s.groupRepo.GetActiveByName(ctx, groupName); lookupErr == nil && existing != nil {
				if existing.RateMultiplier != effectiveMultiplier {
					existing.RateMultiplier = effectiveMultiplier
					_ = s.groupRepo.Update(ctx, existing)
				}
				return existing.ID, nil
			}
			// 查找失败，尝试加 key 前缀去重
			dedupName := fmt.Sprintf("%s [%s]", groupName, res.UpstreamKeyPrefix)
			group.Name = dedupName
			if err2 := s.groupRepo.Create(ctx, group); err2 != nil {
				// 去重名也存在，查找并复用
				if existing, lookupErr := s.groupRepo.GetActiveByName(ctx, dedupName); lookupErr == nil && existing != nil {
					if existing.RateMultiplier != effectiveMultiplier {
						existing.RateMultiplier = effectiveMultiplier
						_ = s.groupRepo.Update(ctx, existing)
					}
					return existing.ID, nil
				}
				return 0, fmt.Errorf("create group (dedup): %w", err2)
			}
			return group.ID, nil
		}
		return 0, fmt.Errorf("create group: %w", err)
	}
	return group.ID, nil
}

// ensureAccount 确保账号存在（幂等），并更新 model_mapping
func (s *UpstreamSyncService) ensureAccount(ctx context.Context, site *UpstreamSite, res *UpstreamManagedResource, existing *UpstreamManagedResource, groupID int64, models []UpstreamModelInfo) (int64, error) {
	// 用该 key 对应的 API Key 作为凭证
	apiKey := res.APIKey
	modelMapping := make(map[string]any, len(models))
	for _, m := range models {
		modelMapping[m.ID] = m.ID
	}

	credentials := map[string]any{
		"api_key":       apiKey,
		"base_url":      strings.TrimRight(site.BaseURL, "/"),
		"model_mapping": modelMapping,
		"site_type":     site.SiteType,
	}

	// 如果已有 managed_account_id，更新凭证和名称
	if existing != nil && existing.ManagedAccountID != nil {
		existingAccount, err := s.adminService.GetAccount(ctx, *existing.ManagedAccountID)
		if err == nil && existingAccount != nil {
			// 保留用户手动修改的 base_url
			existingBaseURL := existingAccount.GetCredential("base_url")
			if existingBaseURL != "" && existingBaseURL != strings.TrimRight(site.BaseURL, "/") {
				credentials["base_url"] = existingBaseURL
			}
			accountName := fmt.Sprintf("上游: %s", site.Name)
			if res.UpstreamKeyName != "" && res.UpstreamKeyName != "Manual API Key" {
				accountName = fmt.Sprintf("%s (%s)", res.UpstreamKeyName, site.Name)
			}
			updateInput := &UpdateAccountInput{
				Name:        accountName,
				Type:        AccountTypeAPIKey,
				Credentials: credentials,
			}
			if _, err := s.adminService.UpdateAccount(ctx, existingAccount.ID, updateInput); err != nil {
				log.Printf("[UpstreamSync] Warning: failed to update account: %v", err)
			}
			return existingAccount.ID, nil
		}
		log.Printf("[UpstreamSync] Managed account %d not found, will recreate", *existing.ManagedAccountID)
	}

	// 创建新账号
	accountName := fmt.Sprintf("上游: %s", site.Name)
	if res.UpstreamKeyName != "" && res.UpstreamKeyName != "Manual API Key" {
		accountName = fmt.Sprintf("%s (%s)", res.UpstreamKeyName, site.Name)
	}

	concurrency := 5
	createInput := &CreateAccountInput{
		Name:                  accountName,
		Platform:              PlatformAntigravity,
		Type:                  AccountTypeAPIKey,
		Credentials:           credentials,
		Concurrency:           concurrency,
		Priority:              0,
		GroupIDs:              []int64{groupID},
		SkipMixedChannelCheck: true,
	}
	account, err := s.adminService.CreateAccount(ctx, createInput)
	if err != nil {
		if strings.Contains(err.Error(), "EXISTS") || strings.Contains(err.Error(), "unique") {
			createInput.Name = fmt.Sprintf("%s [%s]", accountName, res.UpstreamKeyPrefix)
			account, err = s.adminService.CreateAccount(ctx, createInput)
			if err != nil {
				return 0, fmt.Errorf("create account (dedup): %w", err)
			}
			return account.ID, nil
		}
		return 0, fmt.Errorf("create account: %w", err)
	}
	return account.ID, nil
}

// ensureChannel 确保渠道存在（幂等），并更新模型定价
func (s *UpstreamSyncService) ensureChannel(ctx context.Context, site *UpstreamSite, res *UpstreamManagedResource, existing *UpstreamManagedResource, groupID int64, models []UpstreamModelInfo, effectiveMultiplier float64) (int64, error) {
	pricingList := s.buildModelPricing(models, effectiveMultiplier)

	// 如果已有 managed_channel_id，更新定价、名称和模型限制
	if existing != nil && existing.ManagedChannelID != nil {
		existingChannel, err := s.channelService.GetByID(ctx, *existing.ManagedChannelID)
		if err == nil && existingChannel != nil {
			channelName := fmt.Sprintf("上游: %s", site.Name)
			if res.UpstreamKeyName != "" && res.UpstreamKeyName != "Manual API Key" {
				channelName = fmt.Sprintf("%s (%s)", res.UpstreamKeyName, site.Name)
			}
			restrictModels := true
			updateInput := &UpdateChannelInput{
				Name:               channelName,
				ModelPricing:       &pricingList,
				BillingModelSource: BillingModelSourceChannelMapped,
				RestrictModels:     &restrictModels,
			}
			if _, err := s.channelService.Update(ctx, existingChannel.ID, updateInput); err != nil {
				log.Printf("[UpstreamSync] Warning: failed to update channel: %v", err)
			}
			return existingChannel.ID, nil
		}
		log.Printf("[UpstreamSync] Managed channel %d not found, will recreate", *existing.ManagedChannelID)
	}

	// 创建新渠道
	channelName := fmt.Sprintf("上游: %s", site.Name)
	if res.UpstreamKeyName != "" && res.UpstreamKeyName != "Manual API Key" {
		channelName = fmt.Sprintf("%s (%s)", res.UpstreamKeyName, site.Name)
	}

	createInput := &CreateChannelInput{
		Name:               channelName,
		Description:        "",
		GroupIDs:           []int64{groupID},
		ModelPricing:       pricingList,
		BillingModelSource: BillingModelSourceChannelMapped,
		RestrictModels:     true,
	}
	channel, err := s.channelService.Create(ctx, createInput)
	if err != nil {
		if strings.Contains(err.Error(), "EXISTS") || strings.Contains(err.Error(), "unique") {
			createInput.Name = fmt.Sprintf("%s [%s]", channelName, res.UpstreamKeyPrefix)
			channel, err = s.channelService.Create(ctx, createInput)
			if err != nil {
				return 0, fmt.Errorf("create channel (dedup): %w", err)
			}
			return channel.ID, nil
		}
		return 0, fmt.Errorf("create channel: %w", err)
	}
	return channel.ID, nil
}

// ── 模型获取 ──

// fetchModelsWithKey 使用指定的 API Key 获取模型列表
func (s *UpstreamSyncService) fetchModelsWithKey(ctx context.Context, site *UpstreamSite, apiKey string) ([]UpstreamModelInfo, error) {
	if site.SiteType == "grsai" {
		return s.fetchGrsaiModels(ctx, site, apiKey)
	}

	url := strings.TrimRight(site.BaseURL, "/") + "/v1/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

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
	// Deduplicate models by ID (some upstreams return duplicates)
	seen := make(map[string]struct{}, len(result.Data))
	deduped := make([]UpstreamModelInfo, 0, len(result.Data))
	for _, m := range result.Data {
		if _, ok := seen[m.ID]; ok {
			continue
		}
		seen[m.ID] = struct{}{}
		deduped = append(deduped, m)
	}
	return deduped, nil
}

// fetchGrsaiModels grsai 没有 /v1/models 端点，通过验证 API Key 有效性来确认，返回已知模型列表
func (s *UpstreamSyncService) fetchGrsaiModels(ctx context.Context, site *UpstreamSite, apiKey string) ([]UpstreamModelInfo, error) {
	// 用 getCredits 接口验证 API Key 是否有效
	url := strings.TrimRight(site.BaseURL, "/") + "/client/common/getCredits"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("grsai returned %d: %s", resp.StatusCode, string(body))
	}

	return []UpstreamModelInfo{
		{ID: "gpt-image-2", Type: "image", DisplayName: "GPT Image 2"},
	}, nil
}

// ── 余额查询 ──

// fetchBalanceViaJWT login 模式: GET /api/v1/auth/me → balance
func (s *UpstreamSyncService) fetchBalanceViaJWT(ctx context.Context, site *UpstreamSite, accessToken string) (*UpstreamBalanceInfo, error) {
	url := strings.TrimRight(site.BaseURL, "/") + "/api/v1/auth/me"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("upstream returned %d: %s", resp.StatusCode, string(body))
	}

	// /api/v1/auth/me 响应格式: {code: 0, message: "success", data: {id, email, balance, ...}}
	var result struct {
		Code int `json:"code"`
		Data struct {
			Balance float64 `json:"balance"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &UpstreamBalanceInfo{
		BalanceUSD:   result.Data.Balance,
		RemainingUSD: result.Data.Balance,
	}, nil
}

// fetchBalanceViaAPIKey api_key 模式: GET /v1/usage
func (s *UpstreamSyncService) fetchBalanceViaAPIKey(ctx context.Context, site *UpstreamSite) (*UpstreamBalanceInfo, error) {
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

// ── 辅助函数 ──

// maskAPIKey 遮蔽 API Key 仅显示前缀
func maskAPIKey(key string) string {
	if len(key) <= 10 {
		return key
	}
	return key[:10] + "..."
}

// buildModelPricing 根据上游模型列表和倍率构建定价
func (s *UpstreamSyncService) buildModelPricing(models []UpstreamModelInfo, multiplier float64) []ChannelModelPricing {
	// Deduplicate models by ID before processing
	seen := make(map[string]struct{}, len(models))
	dedupModels := make([]UpstreamModelInfo, 0, len(models))
	for _, m := range models {
		if _, ok := seen[m.ID]; ok {
			continue
		}
		seen[m.ID] = struct{}{}
		dedupModels = append(dedupModels, m)
	}
	models = dedupModels

	// 分离图片模型和文本模型
	var imageModels []UpstreamModelInfo
	var textModels []UpstreamModelInfo
	for _, m := range models {
		if IsImageModel(m.ID) {
			imageModels = append(imageModels, m)
		} else {
			textModels = append(textModels, m)
		}
	}

	var pricingList []ChannelModelPricing

	// 处理文本模型：按平台分组，按 token 计费
	platformModels := make(map[string][]string)
	for _, m := range textModels {
		platform := detectPlatform(m.ID)
		platformModels[platform] = append(platformModels[platform], m.ID)
	}
	for platform, modelNames := range platformModels {
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

	// 处理图片模型：按次计费 (BillingModeImage)
	for _, m := range imageModels {
		platform := detectPlatform(m.ID)
		pricing := ChannelModelPricing{
			Platform:    platform,
			Models:      []string{m.ID},
			BillingMode: BillingModeImage,
		}
		if price, ok := LookupImageModelPrice(m.ID); ok {
			p := price * multiplier
			pricing.PerRequestPrice = &p
		} else {
			p := defaultImageFallbackPrice * multiplier
			pricing.PerRequestPrice = &p
		}
		pricingList = append(pricingList, pricing)
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
var defaultModelPrices = map[string]modelDefaultPrice{
	// Claude 系列 (Anthropic)
	"claude-opus-4-7-thinking":   {InputPerToken: 5e-6, OutputPerToken: 25e-6, CacheWritePerToken: ptrFloat(6.25e-6), CacheReadPerToken: ptrFloat(0.50e-6)},
	"claude-opus-4-7":            {InputPerToken: 5e-6, OutputPerToken: 25e-6, CacheWritePerToken: ptrFloat(6.25e-6), CacheReadPerToken: ptrFloat(0.50e-6)},
	"claude-opus-4-6-thinking":   {InputPerToken: 15e-6, OutputPerToken: 75e-6, CacheWritePerToken: ptrFloat(18.75e-6), CacheReadPerToken: ptrFloat(1.50e-6)},
	"claude-opus-4-5-thinking":   {InputPerToken: 15e-6, OutputPerToken: 75e-6, CacheWritePerToken: ptrFloat(18.75e-6), CacheReadPerToken: ptrFloat(1.50e-6)},
	"claude-sonnet-4-6":          {InputPerToken: 3e-6, OutputPerToken: 15e-6, CacheWritePerToken: ptrFloat(3.75e-6), CacheReadPerToken: ptrFloat(0.30e-6)},
	"claude-sonnet-4-5":          {InputPerToken: 3e-6, OutputPerToken: 15e-6, CacheWritePerToken: ptrFloat(3.75e-6), CacheReadPerToken: ptrFloat(0.30e-6)},
	"claude-sonnet-4-5-thinking": {InputPerToken: 3e-6, OutputPerToken: 15e-6, CacheWritePerToken: ptrFloat(3.75e-6), CacheReadPerToken: ptrFloat(0.30e-6)},
	"claude-haiku-4-5":           {InputPerToken: 0.80e-6, OutputPerToken: 4e-6, CacheWritePerToken: ptrFloat(1e-6), CacheReadPerToken: ptrFloat(0.08e-6)},

	// GPT 系列 (OpenAI)
	"gpt-4o":               {InputPerToken: 2.5e-6, OutputPerToken: 10e-6},
	"gpt-4o-mini":          {InputPerToken: 0.15e-6, OutputPerToken: 0.60e-6},
	"gpt-4-turbo":          {InputPerToken: 10e-6, OutputPerToken: 30e-6},
	"gpt-5.4":              {InputPerToken: 2.5e-6, OutputPerToken: 15e-6},
	"gpt-5.4-mini":         {InputPerToken: 0.75e-6, OutputPerToken: 4.5e-6},
	"gpt-5.5":              {InputPerToken: 5e-6, OutputPerToken: 30e-6},
	"gpt-5.5-pro":          {InputPerToken: 30e-6, OutputPerToken: 180e-6},
	"o1":                   {InputPerToken: 15e-6, OutputPerToken: 60e-6},
	"o1-mini":              {InputPerToken: 3e-6, OutputPerToken: 12e-6},
	"o3":                   {InputPerToken: 10e-6, OutputPerToken: 40e-6},
	"o3-mini":              {InputPerToken: 1.1e-6, OutputPerToken: 4.4e-6},
	"o4-mini":              {InputPerToken: 1.1e-6, OutputPerToken: 4.4e-6},
	"gpt-oss-120b-medium":  {InputPerToken: 2e-6, OutputPerToken: 8e-6},

	// Gemini 系列 (Google)
	"gemini-2.5-pro":      {InputPerToken: 1.25e-6, OutputPerToken: 10e-6},
	"gemini-2.5-flash":    {InputPerToken: 0.15e-6, OutputPerToken: 0.60e-6},
	"gemini-3-flash":      {InputPerToken: 0.15e-6, OutputPerToken: 0.60e-6},
	"gemini-3-pro-high":   {InputPerToken: 1.25e-6, OutputPerToken: 10e-6},
	"gemini-3-pro-low":    {InputPerToken: 0.625e-6, OutputPerToken: 5e-6},
	"gemini-3.1-pro-high": {InputPerToken: 1.25e-6, OutputPerToken: 10e-6},
	"gemini-3.1-pro-low":  {InputPerToken: 0.625e-6, OutputPerToken: 5e-6},
}

// defaultImageModelPrices 图片模型默认按次计费价格（USD per request）
var defaultImageModelPrices = map[string]float64{
	"gpt-image-1":  0.080,
	"gpt-image-2":  0.080,
	"dall-e-3":     0.040,
	"dall-e-2":     0.020,
}

const defaultImageFallbackPrice = 0.080

// IsImageModel 判断模型是否为图片生成模型
func IsImageModel(modelID string) bool {
	lower := strings.ToLower(modelID)
	if strings.HasPrefix(lower, "gpt-image") || strings.HasPrefix(lower, "dall-e") {
		return true
	}
	return false
}

// LookupImageModelPrice 查找图片模型按次价格（精确匹配 → 前缀匹配）
func LookupImageModelPrice(model string) (float64, bool) {
	lower := strings.ToLower(model)
	if p, ok := defaultImageModelPrices[lower]; ok {
		return p, true
	}
	bestMatch := ""
	for key := range defaultImageModelPrices {
		if strings.HasPrefix(lower, key) && len(key) > len(bestMatch) {
			bestMatch = key
		}
	}
	if bestMatch != "" {
		return defaultImageModelPrices[bestMatch], true
	}
	return 0, false
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
	seen := make(map[string]struct{}, len(modelNames))

	for _, name := range modelNames {
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
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
				Models:      []string{name},
				InputPrice:  &inputP,
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

	// 未知模型：不设价格
	if len(unknownModels) > 0 {
		result = append(result, pricingGroup{
			Models: unknownModels,
		})
	}
	return result
}

// lookupModelPrice 查找模型默认价格（精确匹配 → 前缀匹配）
func lookupModelPrice(model string) (modelDefaultPrice, bool) {
	if p, ok := defaultModelPrices[model]; ok {
		return p, true
	}
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
		strings.HasPrefix(lower, "o4"), strings.HasPrefix(lower, "chatgpt"), strings.HasPrefix(lower, "dall-e"):
		return PlatformOpenAI
	case strings.HasPrefix(lower, "gemini"), strings.HasPrefix(lower, "tab_"):
		return PlatformGemini
	default:
		return PlatformAntigravity
	}
}

// upstreamPriceTextRe matches upstream price text like "价格 1k:0.06 2k:0.12 4k:0.24"
var upstreamPriceTextRe = regexp.MustCompile(`\s*价格\s+[\dkK:.]+(?:\s+[\dkK:.]+)*\s*`)

// stripUpstreamPriceText removes upstream price text from description.
func stripUpstreamPriceText(desc string) string {
	return strings.TrimSpace(upstreamPriceTextRe.ReplaceAllString(desc, ""))
}
