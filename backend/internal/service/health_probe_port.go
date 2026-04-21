package service

import (
	"context"
	"time"
)

// HealthProbeConfig represents the single-row health probe configuration.
type HealthProbeConfig struct {
	ID                     int64     `json:"id"`
	Enabled                bool      `json:"enabled"`
	IntervalMinutes        int       `json:"interval_minutes"`
	TimeoutSeconds         int       `json:"timeout_seconds"`
	RetentionHours         int       `json:"retention_hours"`
	SlowThresholdMs        int       `json:"slow_threshold_ms"`
	WebhookEnabled         bool      `json:"webhook_enabled"`
	WebhookURL             string    `json:"webhook_url"`
	WebhookDebounceCount   int       `json:"webhook_debounce_count"`
	WebhookCooldownMinutes int       `json:"webhook_cooldown_minutes"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// HealthProbeResult represents a single health probe execution result.
type HealthProbeResult struct {
	ID             int64     `json:"id"`
	AccountID      int64     `json:"account_id"`
	GroupID        int64     `json:"group_id"`
	ProbeModel     string    `json:"probe_model"`
	Status         int       `json:"status"` // 0=unavailable, 1=available, 2=degraded, 3=rate_limited
	LatencyMs      int       `json:"latency_ms"`
	ErrorType      string    `json:"error_type"` // empty, network_error, auth_error, rate_limit, server_error, timeout
	HttpStatusCode int       `json:"http_status_code"`
	ErrorMessage   string    `json:"error_message"`
	CheckedAt      time.Time `json:"checked_at"`

	// Transient fields for API response enrichment (not stored in DB)
	GroupName      string  `json:"group_name,omitempty"`
	RateMultiplier float64 `json:"rate_multiplier,omitempty"`
	Platform       string  `json:"platform,omitempty"`
}

// HealthProbeSummary represents an aggregated summary per 30-min bucket.
type HealthProbeSummary struct {
	ID              int64     `json:"id"`
	GroupID         int64     `json:"group_id"`
	BucketTime      time.Time `json:"bucket_time"`
	TotalProbes     int       `json:"total_probes"`
	SuccessCount    int       `json:"success_count"`
	AvgLatencyMs    int       `json:"avg_latency_ms"`
	AvailabilityPct float32   `json:"availability_pct"`
	CreatedAt       time.Time `json:"created_at"`
}

// HealthProbeGroupConfig stores per-group probe model override.
type HealthProbeGroupConfig struct {
	ID         int64     `json:"id"`
	GroupID    int64     `json:"group_id"`
	ProbeModel string    `json:"probe_model"` // empty means auto-select
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// HealthProbeConfigRepository defines the data access interface for health probe configuration.
type HealthProbeConfigRepository interface {
	EnsureTable(ctx context.Context) error
	GetConfig(ctx context.Context) (*HealthProbeConfig, error)
	UpdateConfig(ctx context.Context, cfg *HealthProbeConfig) error
}

// HealthProbeResultRepository defines the data access interface for health probe results.
type HealthProbeResultRepository interface {
	EnsureTable(ctx context.Context) error
	Create(ctx context.Context, result *HealthProbeResult) error
	ListByGroup(ctx context.Context, groupID int64, since time.Time, limit int) ([]*HealthProbeResult, error)
	ListLatestByGroups(ctx context.Context) ([]*HealthProbeResult, error)
	Prune(ctx context.Context, before time.Time) (int64, error)
}

// HealthProbeSummaryRepository defines the data access interface for health probe summaries.
type HealthProbeSummaryRepository interface {
	EnsureTable(ctx context.Context) error
	Upsert(ctx context.Context, summary *HealthProbeSummary) error
	ListByGroup(ctx context.Context, groupID int64, since time.Time) ([]*HealthProbeSummary, error)
	ListAllGroups(ctx context.Context, since time.Time) ([]*HealthProbeSummary, error)
}

// HealthProbeGroupConfigRepository defines the data access interface for per-group probe config.
type HealthProbeGroupConfigRepository interface {
	EnsureTable(ctx context.Context) error
	Get(ctx context.Context, groupID int64) (*HealthProbeGroupConfig, error)
	ListAll(ctx context.Context) ([]*HealthProbeGroupConfig, error)
	Upsert(ctx context.Context, cfg *HealthProbeGroupConfig) error
	Delete(ctx context.Context, groupID int64) error
}
