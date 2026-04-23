package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/robfig/cron/v3"
)

const (
	healthProbeDefaultMaxWorkers = 5
	healthProbeStartupGraceSec   = 60 // seconds to wait before sending webhook alerts after startup
)

// Probe status constants (matches HealthProbeResult.Status)
const (
	ProbeStatusUnavailable  = 0
	ProbeStatusAvailable    = 1
	ProbeStatusDegraded     = 2 // slow
	ProbeStatusRateLimited  = 3
)

// Probe error type constants
const (
	ProbeErrorNone        = ""
	ProbeErrorNetwork     = "network_error"
	ProbeErrorAuth        = "auth_error"
	ProbeErrorRateLimit   = "rate_limit"
	ProbeErrorServer      = "server_error"
	ProbeErrorTimeout     = "timeout"
)

// HealthProbeService performs active health probes against upstream accounts.
type HealthProbeService struct {
	configRepo      HealthProbeConfigRepository
	resultRepo      HealthProbeResultRepository
	summaryRepo     HealthProbeSummaryRepository
	groupConfigRepo HealthProbeGroupConfigRepository
	groupRepo       GroupRepository
	accountRepo     AccountRepository
	channelService  *ChannelService
	cfg             *config.Config

	cron      *cron.Cron
	cronID    cron.EntryID
	mu        sync.Mutex
	startOnce sync.Once
	stopOnce  sync.Once

	startedAt time.Time

	// webhook alert state: groupID -> consecutive failure count
	failureCounts   map[int64]int
	failureCountsMu sync.Mutex

	// webhook cooldown: groupID -> last webhook sent time
	webhookCooldown   map[int64]time.Time
	webhookCooldownMu sync.Mutex

	// previous status per group for change detection
	prevStatus   map[int64]int
	prevStatusMu sync.Mutex

	httpClient *http.Client
}

// NewHealthProbeService creates a new HealthProbeService.
func NewHealthProbeService(
	configRepo HealthProbeConfigRepository,
	resultRepo HealthProbeResultRepository,
	summaryRepo HealthProbeSummaryRepository,
	groupConfigRepo HealthProbeGroupConfigRepository,
	groupRepo GroupRepository,
	accountRepo AccountRepository,
	channelService *ChannelService,
	cfg *config.Config,
) *HealthProbeService {
	return &HealthProbeService{
		configRepo:      configRepo,
		resultRepo:      resultRepo,
		summaryRepo:     summaryRepo,
		groupConfigRepo: groupConfigRepo,
		groupRepo:       groupRepo,
		accountRepo:     accountRepo,
		channelService:  channelService,
		cfg:             cfg,
		failureCounts:   make(map[int64]int),
		webhookCooldown: make(map[int64]time.Time),
		prevStatus:      make(map[int64]int),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        20,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     60 * time.Second,
			},
		},
	}
}

// Start begins the background health probe cron.
func (s *HealthProbeService) Start() {
	if s == nil {
		return
	}
	s.startOnce.Do(func() {
		s.startedAt = time.Now()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Ensure tables
		if err := s.configRepo.EnsureTable(ctx); err != nil {
			logger.LegacyPrintf("service.health_probe", "[HealthProbe] EnsureTable(config) error: %v", err)
		}
		if err := s.resultRepo.EnsureTable(ctx); err != nil {
			logger.LegacyPrintf("service.health_probe", "[HealthProbe] EnsureTable(result) error: %v", err)
		}
		if err := s.summaryRepo.EnsureTable(ctx); err != nil {
			logger.LegacyPrintf("service.health_probe", "[HealthProbe] EnsureTable(summary) error: %v", err)
		}
		if err := s.groupConfigRepo.EnsureTable(ctx); err != nil {
			logger.LegacyPrintf("service.health_probe", "[HealthProbe] EnsureTable(groupConfig) error: %v", err)
		}

		// Load config and start cron
		probeCfg, err := s.configRepo.GetConfig(ctx)
		if err != nil {
			logger.LegacyPrintf("service.health_probe", "[HealthProbe] GetConfig error: %v", err)
			return
		}

		if !probeCfg.Enabled {
			logger.LegacyPrintf("service.health_probe", "[HealthProbe] disabled by config")
			return
		}

		s.scheduleCron(probeCfg.IntervalMinutes)
		logger.LegacyPrintf("service.health_probe", "[HealthProbe] started (interval=%dm)", probeCfg.IntervalMinutes)
	})
}

// Stop gracefully shuts down the cron scheduler.
func (s *HealthProbeService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.cron != nil {
			ctx := s.cron.Stop()
			select {
			case <-ctx.Done():
			case <-time.After(3 * time.Second):
				logger.LegacyPrintf("service.health_probe", "[HealthProbe] cron stop timed out")
			}
		}
	})
}

// Reschedule reloads config and reschedules the cron if interval changed.
func (s *HealthProbeService) Reschedule(ctx context.Context) error {
	probeCfg, err := s.configRepo.GetConfig(ctx)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cron != nil {
		ctx2 := s.cron.Stop()
		select {
		case <-ctx2.Done():
		case <-time.After(3 * time.Second):
		}
		s.cron = nil
	}

	if !probeCfg.Enabled {
		logger.LegacyPrintf("service.health_probe", "[HealthProbe] disabled by config (reschedule)")
		return nil
	}

	// Update http client timeout
	s.httpClient.Timeout = time.Duration(probeCfg.TimeoutSeconds) * time.Second

	s.scheduleCron(probeCfg.IntervalMinutes)
	logger.LegacyPrintf("service.health_probe", "[HealthProbe] rescheduled (interval=%dm)", probeCfg.IntervalMinutes)
	return nil
}

func (s *HealthProbeService) scheduleCron(intervalMinutes int) {
	loc := time.Local
	if s.cfg != nil {
		if parsed, err := time.LoadLocation(s.cfg.Timezone); err == nil && parsed != nil {
			loc = parsed
		}
	}

	c := cron.New(cron.WithParser(cron.NewParser(
		cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
	)), cron.WithLocation(loc))

	spec := fmt.Sprintf("*/%d * * * *", intervalMinutes)
	id, err := c.AddFunc(spec, func() { s.runProbeRound() })
	if err != nil {
		logger.LegacyPrintf("service.health_probe", "[HealthProbe] failed to add cron: %v", err)
		return
	}
	s.cron = c
	s.cronID = id
	s.cron.Start()
}

// RunManualProbe triggers one probe round immediately (for admin manual trigger).
func (s *HealthProbeService) RunManualProbe() {
	go s.runProbeRound()
}

// GetConfig returns the current health probe configuration.
func (s *HealthProbeService) GetConfig(ctx context.Context) (*HealthProbeConfig, error) {
	return s.configRepo.GetConfig(ctx)
}

// UpdateConfig updates the health probe configuration.
func (s *HealthProbeService) UpdateConfig(ctx context.Context, cfg *HealthProbeConfig) error {
	if err := s.configRepo.UpdateConfig(ctx, cfg); err != nil {
		return err
	}
	return s.Reschedule(ctx)
}

// GetGroupResults returns probe results for a specific group.
func (s *HealthProbeService) GetGroupResults(ctx context.Context, groupID int64, hours int, limit int) ([]*HealthProbeResult, error) {
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	return s.resultRepo.ListByGroup(ctx, groupID, since, limit)
}

// GetLatestResults returns the latest probe result per group.
func (s *HealthProbeService) GetLatestResults(ctx context.Context) ([]*HealthProbeResult, error) {
	return s.resultRepo.ListLatestByGroups(ctx)
}

// GetLatestResultsForUsers returns the latest probe results for users.
// Groups with probing disabled (probe_enabled=false) are shown as "available" with synthetic results.
func (s *HealthProbeService) GetLatestResultsForUsers(ctx context.Context) ([]*HealthProbeResult, error) {
	results, err := s.resultRepo.ListLatestByGroups(ctx)
	if err != nil {
		return nil, err
	}

	// Load group configs to find disabled groups
	allGroupCfgs, _ := s.groupConfigRepo.ListAll(ctx)
	disabledGroups := make(map[int64]bool)
	for _, gc := range allGroupCfgs {
		if !gc.IsProbeEnabled() {
			disabledGroups[gc.GroupID] = true
		}
	}

	// Build set of groups already in results
	probedGroupIDs := make(map[int64]bool, len(results))
	for _, r := range results {
		probedGroupIDs[r.GroupID] = true
	}

	// Get all active groups to add synthetic results for disabled ones
	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return results, nil // return what we have
	}

	for _, g := range groups {
		if disabledGroups[g.ID] && !probedGroupIDs[g.ID] {
			// Add synthetic "available" result for unprobed group
			results = append(results, &HealthProbeResult{
				GroupID:    g.ID,
				Status:     ProbeStatusAvailable,
				LatencyMs:  0,
				ProbeModel: "",
				CheckedAt:  time.Now(),
			})
		}
	}

	return results, nil
}

// GetGroupSummaries returns aggregated summaries for a group.
func (s *HealthProbeService) GetGroupSummaries(ctx context.Context, groupID int64, hours int) ([]*HealthProbeSummary, error) {
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	return s.summaryRepo.ListByGroup(ctx, groupID, since)
}

// GetAllSummaries returns aggregated summaries for all groups.
func (s *HealthProbeService) GetAllSummaries(ctx context.Context, hours int) ([]*HealthProbeSummary, error) {
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	return s.summaryRepo.ListAllGroups(ctx, since)
}

// --- probe execution ---

func (s *HealthProbeService) runProbeRound() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	probeCfg, err := s.configRepo.GetConfig(ctx)
	if err != nil {
		logger.LegacyPrintf("service.health_probe", "[HealthProbe] GetConfig error in round: %v", err)
		return
	}
	if !probeCfg.Enabled {
		return
	}

	// Update http client timeout per config
	s.httpClient.Timeout = time.Duration(probeCfg.TimeoutSeconds) * time.Second

	// Get all active groups
	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		logger.LegacyPrintf("service.health_probe", "[HealthProbe] ListActive groups error: %v", err)
		return
	}

	if len(groups) == 0 {
		return
	}

	logger.LegacyPrintf("service.health_probe", "[HealthProbe] starting probe round for %d groups", len(groups))

	sem := make(chan struct{}, healthProbeDefaultMaxWorkers)
	var wg sync.WaitGroup

	// Load per-group configs to check probe_enabled
	allGroupCfgs, _ := s.groupConfigRepo.ListAll(ctx)
	disabledGroups := make(map[int64]bool)
	for _, gc := range allGroupCfgs {
		if !gc.IsProbeEnabled() {
			disabledGroups[gc.GroupID] = true
		}
	}

	for i := range groups {
		group := &groups[i]
		if disabledGroups[group.ID] {
			continue // skip probing for disabled groups
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(g *Group) {
			defer wg.Done()
			defer func() { <-sem }()
			s.probeGroup(ctx, g, probeCfg)
		}(group)
	}
	wg.Wait()

	// After probing, aggregate summaries and prune old data
	s.aggregateSummaries(ctx, groups)
	s.pruneOldData(ctx, probeCfg)

	logger.LegacyPrintf("service.health_probe", "[HealthProbe] probe round complete")
}

func (s *HealthProbeService) probeGroup(ctx context.Context, group *Group, probeCfg *HealthProbeConfig) {
	// Get accounts for this group
	accounts, err := s.accountRepo.ListByGroup(ctx, group.ID)
	if err != nil {
		logger.LegacyPrintf("service.health_probe", "[HealthProbe] ListByGroup(%d) error: %v", group.ID, err)
		return
	}

	if len(accounts) == 0 {
		return
	}

	// Pick the best account to probe (first active one)
	var probeAccount *Account
	for i := range accounts {
		a := &accounts[i]
		if a.IsActive() && a.Schedulable {
			probeAccount = a
			break
		}
	}

	now := time.Now()

	if probeAccount == nil {
		// All accounts disabled/inactive — record as unavailable
		result := &HealthProbeResult{
			AccountID:  0,
			GroupID:    group.ID,
			ProbeModel: "",
			Status:     ProbeStatusUnavailable,
			LatencyMs:  0,
			ErrorType:  ProbeErrorNone,
			ErrorMessage: "no active accounts in group",
			CheckedAt:  now,
		}
		_ = s.resultRepo.Create(ctx, result)
		s.handleStatusChange(ctx, group.ID, ProbeStatusUnavailable, probeCfg)
		return
	}

	// Select probe model from account's model_mapping
	probeModel := s.selectProbeModel(probeAccount, group)

	// Execute the actual probe
	result := s.executeProbe(ctx, probeAccount, group, probeModel, probeCfg)
	result.CheckedAt = now

	// Save result
	if err := s.resultRepo.Create(ctx, result); err != nil {
		logger.LegacyPrintf("service.health_probe", "[HealthProbe] Create result error: %v", err)
	}

	// Handle webhook alerts
	s.handleStatusChange(ctx, group.ID, result.Status, probeCfg)
}

func (s *HealthProbeService) selectProbeModel(account *Account, group *Group) string {
	// Check per-group config first
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	groupCfg, err := s.groupConfigRepo.Get(ctx, group.ID)
	if err == nil && groupCfg != nil && groupCfg.ProbeModel != "" {
		return groupCfg.ProbeModel
	}

	// Platform-aware preferred models
	var preferredModels []string
	switch group.Platform {
	case PlatformGemini:
		preferredModels = []string{
			"gemini-2.0-flash",
			"gemini-1.5-flash",
		}
	case PlatformOpenAI:
		preferredModels = []string{
			"gpt-4o-mini",
			"gpt-3.5-turbo",
		}
	default:
		// Claude / Antigravity / others
		preferredModels = []string{
			"claude-haiku-4-5-20250514",
			"claude-3-5-haiku-20241022",
			"claude-3-haiku-20240307",
			"gpt-4o-mini",
			"gpt-3.5-turbo",
			"gemini-2.0-flash",
			"gemini-1.5-flash",
		}
	}

	mapping := account.GetModelMapping()
	if mapping != nil {
		for _, m := range preferredModels {
			if _, ok := mapping[m]; ok {
				return m
			}
		}
		// Just use the first mapped model
		for k := range mapping {
			return k
		}
	}

	// Default by platform
	switch group.Platform {
	case PlatformOpenAI:
		return "gpt-4o-mini"
	case PlatformGemini:
		return "gemini-2.0-flash"
	case PlatformAntigravity:
		return "claude-3-5-haiku-20241022"
	default:
		return "claude-3-5-haiku-20241022"
	}
}

func (s *HealthProbeService) executeProbe(ctx context.Context, account *Account, group *Group, model string, probeCfg *HealthProbeConfig) *HealthProbeResult {
	result := &HealthProbeResult{
		AccountID:  account.ID,
		GroupID:    group.ID,
		ProbeModel: model,
	}

	// Apply model mapping if exists
	mappedModel := account.GetMappedModel(model)
	if mappedModel != "" {
		model = mappedModel
	}

	// Build and execute request based on platform (with stream=true)
	var req *http.Request
	var err error

	switch {
	case account.IsOpenAI():
		req, err = s.buildOpenAIProbeRequest(ctx, account, model)
	case account.IsGemini():
		req, err = s.buildGeminiProbeRequest(ctx, account, model)
	case account.Platform == PlatformAntigravity:
		req, err = s.buildClaudeProbeRequest(ctx, account, model)
	default:
		// Anthropic / apikey / upstream — use Claude protocol
		req, err = s.buildClaudeProbeRequest(ctx, account, model)
	}

	if err != nil {
		result.Status = ProbeStatusUnavailable
		result.ErrorType = ProbeErrorNetwork
		result.ErrorMessage = fmt.Sprintf("build request failed: %v", err)
		return result
	}

	// Use a custom transport that does NOT auto-decompress, to get raw streaming bytes
	start := time.Now()
	resp, err := s.httpClient.Do(req)

	if err != nil {
		result.LatencyMs = int(time.Since(start).Milliseconds())
		result.Status = ProbeStatusUnavailable
		if ctx.Err() != nil || strings.Contains(err.Error(), "deadline exceeded") || strings.Contains(err.Error(), "timeout") {
			result.ErrorType = ProbeErrorTimeout
			result.ErrorMessage = "request timed out"
		} else {
			result.ErrorType = ProbeErrorNetwork
			result.ErrorMessage = fmt.Sprintf("request failed: %v", err)
		}
		return result
	}
	defer func() { _ = resp.Body.Close() }()

	result.HttpStatusCode = resp.StatusCode

	if resp.StatusCode != 200 {
		result.LatencyMs = int(time.Since(start).Milliseconds())
		// Read error body
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		switch {
		case resp.StatusCode == 429:
			result.Status = ProbeStatusRateLimited
			result.ErrorType = ProbeErrorRateLimit
			result.ErrorMessage = "rate limited"
		case resp.StatusCode == 401 || resp.StatusCode == 403:
			result.Status = ProbeStatusUnavailable
			result.ErrorType = ProbeErrorAuth
			result.ErrorMessage = extractErrorMessage(bodyBytes)
		case resp.StatusCode >= 500:
			result.Status = ProbeStatusUnavailable
			result.ErrorType = ProbeErrorServer
			result.ErrorMessage = extractErrorMessage(bodyBytes)
		default:
			result.Status = ProbeStatusUnavailable
			result.ErrorType = ProbeErrorServer
			result.ErrorMessage = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, extractErrorMessage(bodyBytes))
		}
		return result
	}

	// Streaming TTFB: read until we get the first data chunk
	buf := make([]byte, 256)
	_, err = resp.Body.Read(buf)
	ttfb := time.Since(start)
	result.LatencyMs = int(ttfb.Milliseconds())

	if err != nil && err != io.EOF {
		result.Status = ProbeStatusUnavailable
		result.ErrorType = ProbeErrorNetwork
		result.ErrorMessage = fmt.Sprintf("stream read failed: %v", err)
		return result
	}

	// Success — check for slow response (degraded)
	result.Status = ProbeStatusAvailable
	result.ErrorType = ProbeErrorNone
	if result.LatencyMs > probeCfg.SlowThresholdMs {
		result.Status = ProbeStatusDegraded
	}

	return result
}

// --- platform-specific probe request builders ---

func (s *HealthProbeService) buildClaudeProbeRequest(ctx context.Context, account *Account, model string) (*http.Request, error) {
	body := map[string]any{
		"model":      model,
		"max_tokens": 1,
		"stream":     true,
		"messages": []map[string]string{
			{"role": "user", "content": "hi"},
		},
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	// Determine base URL
	baseURL := account.GetCredential("base_url")
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	url := baseURL + "/v1/messages"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	// Auth
	apiKey := account.GetCredential("api_key")
	if apiKey != "" {
		req.Header.Set("x-api-key", apiKey)
	} else {
		accessToken := account.GetCredential("access_token")
		if accessToken != "" {
			req.Header.Set("Authorization", "Bearer "+accessToken)
		}
	}

	return req, nil
}

func (s *HealthProbeService) buildOpenAIProbeRequest(ctx context.Context, account *Account, model string) (*http.Request, error) {
	body := map[string]any{
		"model":      model,
		"max_tokens": 1,
		"stream":     true,
		"messages": []map[string]string{
			{"role": "user", "content": "hi"},
		},
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	baseURL := account.GetCredential("base_url")
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	url := baseURL + "/v1/chat/completions"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	apiKey := account.GetCredential("api_key")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	} else {
		accessToken := account.GetCredential("access_token")
		if accessToken != "" {
			req.Header.Set("Authorization", "Bearer "+accessToken)
		}
	}

	return req, nil
}

func (s *HealthProbeService) buildGeminiProbeRequest(ctx context.Context, account *Account, model string) (*http.Request, error) {
	// Gemini uses a different API structure
	body := map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]string{
					{"text": "hi"},
				},
			},
		},
		"generationConfig": map[string]any{
			"maxOutputTokens": 1,
		},
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	apiKey := account.GetCredential("api_key")
	baseURL := "https://generativelanguage.googleapis.com"
	url := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s", baseURL, model, apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

// --- group config CRUD ---

// GetGroupConfig returns the probe config for a specific group.
func (s *HealthProbeService) GetGroupConfig(ctx context.Context, groupID int64) (*HealthProbeGroupConfig, error) {
	return s.groupConfigRepo.Get(ctx, groupID)
}

// ListGroupConfigs returns all per-group probe configs.
func (s *HealthProbeService) ListGroupConfigs(ctx context.Context) ([]*HealthProbeGroupConfig, error) {
	return s.groupConfigRepo.ListAll(ctx)
}

// UpsertGroupConfig creates or updates the probe config for a group.
func (s *HealthProbeService) UpsertGroupConfig(ctx context.Context, cfg *HealthProbeGroupConfig) error {
	return s.groupConfigRepo.Upsert(ctx, cfg)
}

// DeleteGroupConfig removes the probe config for a group.
func (s *HealthProbeService) DeleteGroupConfig(ctx context.Context, groupID int64) error {
	return s.groupConfigRepo.Delete(ctx, groupID)
}

// EnrichResultsWithGroupInfo adds group name, rate multiplier, and platform to results.
func (s *HealthProbeService) EnrichResultsWithGroupInfo(ctx context.Context, results []*HealthProbeResult) {
	for _, r := range results {
		group, err := s.groupRepo.GetByIDLite(ctx, r.GroupID)
		if err == nil && group != nil {
			r.GroupName = group.Name
			r.RateMultiplier = group.RateMultiplier
			r.Platform = group.Platform
			if group.ImagePrice1K != nil && *group.ImagePrice1K > 0 {
				r.BillingDisplay = fmt.Sprintf("$%.3g/次", *group.ImagePrice1K)
			}
		}
	}
}

// GetGroupModels returns available models for each active group.
func (s *HealthProbeService) GetGroupModels(ctx context.Context) (map[int64][]string, error) {
	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[int64][]string, len(groups))
	for _, g := range groups {
		accounts, err := s.accountRepo.ListByGroup(ctx, g.ID)
		if err != nil {
			continue
		}
		modelSet := make(map[string]struct{})
		for i := range accounts {
			for model := range accounts[i].GetModelMapping() {
				modelSet[model] = struct{}{}
			}
		}

		// Apply channel RestrictModels filter (same logic as GetAvailableModels)
		if s.channelService != nil {
			ch, err := s.channelService.GetChannelForGroup(ctx, g.ID)
			if err == nil && ch != nil && ch.RestrictModels && len(ch.ModelPricing) > 0 {
				for model := range modelSet {
					if ch.GetModelPricing(model) == nil {
						delete(modelSet, model)
					}
				}
			}
		}

		models := make([]string, 0, len(modelSet))
		for m := range modelSet {
			models = append(models, m)
		}
		sort.Strings(models)
		result[g.ID] = models
	}
	return result, nil
}

// --- webhook alerts ---

func (s *HealthProbeService) handleStatusChange(ctx context.Context, groupID int64, newStatus int, probeCfg *HealthProbeConfig) {
	if !probeCfg.WebhookEnabled || probeCfg.WebhookURL == "" {
		return
	}

	// Startup grace period
	if time.Since(s.startedAt) < time.Duration(healthProbeStartupGraceSec)*time.Second {
		return
	}

	// Check if status changed
	s.prevStatusMu.Lock()
	oldStatus, existed := s.prevStatus[groupID]
	s.prevStatus[groupID] = newStatus
	s.prevStatusMu.Unlock()

	if !existed {
		// First probe — no comparison yet, just record
		return
	}

	if oldStatus == newStatus {
		// No change — reset failure count if healthy
		if newStatus == ProbeStatusAvailable {
			s.failureCountsMu.Lock()
			s.failureCounts[groupID] = 0
			s.failureCountsMu.Unlock()
		}
		return
	}

	// Status changed — apply debounce for failures
	if newStatus == ProbeStatusUnavailable || newStatus == ProbeStatusDegraded {
		s.failureCountsMu.Lock()
		s.failureCounts[groupID]++
		count := s.failureCounts[groupID]
		s.failureCountsMu.Unlock()

		if count < probeCfg.WebhookDebounceCount {
			return // Not enough consecutive failures
		}
	} else {
		// Recovery — reset failure count
		s.failureCountsMu.Lock()
		s.failureCounts[groupID] = 0
		s.failureCountsMu.Unlock()
	}

	// Check cooldown
	s.webhookCooldownMu.Lock()
	lastSent, hasCooldown := s.webhookCooldown[groupID]
	s.webhookCooldownMu.Unlock()

	if hasCooldown && time.Since(lastSent) < time.Duration(probeCfg.WebhookCooldownMinutes)*time.Minute {
		return
	}

	// Send webhook
	go s.sendWebhook(probeCfg.WebhookURL, groupID, oldStatus, newStatus)

	s.webhookCooldownMu.Lock()
	s.webhookCooldown[groupID] = time.Now()
	s.webhookCooldownMu.Unlock()
}

func (s *HealthProbeService) sendWebhook(webhookURL string, groupID int64, oldStatus, newStatus int) {
	statusName := func(st int) string {
		switch st {
		case ProbeStatusAvailable:
			return "available"
		case ProbeStatusDegraded:
			return "degraded"
		case ProbeStatusRateLimited:
			return "rate_limited"
		default:
			return "unavailable"
		}
	}

	payload := map[string]any{
		"event":      "health_status_change",
		"group_id":   groupID,
		"old_status": statusName(oldStatus),
		"new_status": statusName(newStatus),
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		logger.LegacyPrintf("service.health_probe", "[HealthProbe] webhook marshal error: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(bodyBytes))
	if err != nil {
		logger.LegacyPrintf("service.health_probe", "[HealthProbe] webhook request error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		logger.LegacyPrintf("service.health_probe", "[HealthProbe] webhook send error: %v", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		logger.LegacyPrintf("service.health_probe", "[HealthProbe] webhook returned HTTP %d for group %d", resp.StatusCode, groupID)
	}
}

// --- summary aggregation ---

func (s *HealthProbeService) aggregateSummaries(ctx context.Context, groups []Group) {
	// Aggregate results from the last 30 minutes into summary buckets
	now := time.Now()
	bucketTime := now.Truncate(30 * time.Minute)
	since := bucketTime // Only aggregate current bucket

	for _, group := range groups {
		results, err := s.resultRepo.ListByGroup(ctx, group.ID, since, 1000)
		if err != nil || len(results) == 0 {
			continue
		}

		totalProbes := len(results)
		successCount := 0
		totalLatency := 0
		for _, r := range results {
			if r.Status == ProbeStatusAvailable || r.Status == ProbeStatusDegraded {
				successCount++
			}
			totalLatency += r.LatencyMs
		}

		avgLatency := 0
		if totalProbes > 0 {
			avgLatency = totalLatency / totalProbes
		}

		availPct := float32(0)
		if totalProbes > 0 {
			availPct = float32(successCount) / float32(totalProbes) * 100.0
		}

		summary := &HealthProbeSummary{
			GroupID:         group.ID,
			BucketTime:      bucketTime,
			TotalProbes:     totalProbes,
			SuccessCount:    successCount,
			AvgLatencyMs:    avgLatency,
			AvailabilityPct: availPct,
		}

		if err := s.summaryRepo.Upsert(ctx, summary); err != nil {
			logger.LegacyPrintf("service.health_probe", "[HealthProbe] Upsert summary error for group %d: %v", group.ID, err)
		}
	}
}

// --- data pruning ---

func (s *HealthProbeService) pruneOldData(ctx context.Context, probeCfg *HealthProbeConfig) {
	before := time.Now().Add(-time.Duration(probeCfg.RetentionHours) * time.Hour)
	pruned, err := s.resultRepo.Prune(ctx, before)
	if err != nil {
		logger.LegacyPrintf("service.health_probe", "[HealthProbe] Prune error: %v", err)
		return
	}
	if pruned > 0 {
		logger.LegacyPrintf("service.health_probe", "[HealthProbe] pruned %d old results", pruned)
	}
}

// --- helpers ---

func extractErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	// Try to parse as JSON and extract error message
	var resp struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err == nil && resp.Error.Message != "" {
		msg := resp.Error.Message
		if len(msg) > 200 {
			msg = msg[:200] + "..."
		}
		return msg
	}
	// Fallback: return raw (truncated)
	s := string(body)
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}
