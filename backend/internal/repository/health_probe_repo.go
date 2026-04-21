package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// --- Config Repository ---

type healthProbeConfigRepository struct {
	db *sql.DB
}

func NewHealthProbeConfigRepository(db *sql.DB) service.HealthProbeConfigRepository {
	return &healthProbeConfigRepository{db: db}
}

func (r *healthProbeConfigRepository) EnsureTable(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS health_probe_configs (
			id BIGSERIAL PRIMARY KEY,
			enabled BOOLEAN NOT NULL DEFAULT true,
			interval_minutes INTEGER NOT NULL DEFAULT 30,
			timeout_seconds INTEGER NOT NULL DEFAULT 15,
			retention_hours INTEGER NOT NULL DEFAULT 72,
			slow_threshold_ms INTEGER NOT NULL DEFAULT 10000,
			webhook_enabled BOOLEAN NOT NULL DEFAULT false,
			webhook_url TEXT NOT NULL DEFAULT '',
			webhook_debounce_count INTEGER NOT NULL DEFAULT 2,
			webhook_cooldown_minutes INTEGER NOT NULL DEFAULT 10,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

func (r *healthProbeConfigRepository) GetConfig(ctx context.Context) (*service.HealthProbeConfig, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, enabled, interval_minutes, timeout_seconds, retention_hours, slow_threshold_ms,
		       webhook_enabled, webhook_url, webhook_debounce_count, webhook_cooldown_minutes,
		       created_at, updated_at
		FROM health_probe_configs
		ORDER BY id ASC
		LIMIT 1
	`)

	cfg := &service.HealthProbeConfig{}
	err := row.Scan(
		&cfg.ID, &cfg.Enabled, &cfg.IntervalMinutes, &cfg.TimeoutSeconds,
		&cfg.RetentionHours, &cfg.SlowThresholdMs,
		&cfg.WebhookEnabled, &cfg.WebhookURL, &cfg.WebhookDebounceCount, &cfg.WebhookCooldownMinutes,
		&cfg.CreatedAt, &cfg.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		// Insert default row and return it
		return r.insertDefault(ctx)
	}
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func (r *healthProbeConfigRepository) insertDefault(ctx context.Context) (*service.HealthProbeConfig, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO health_probe_configs (enabled, interval_minutes, timeout_seconds, retention_hours, slow_threshold_ms,
		                                  webhook_enabled, webhook_url, webhook_debounce_count, webhook_cooldown_minutes)
		VALUES (true, 30, 15, 72, 10000, false, '', 2, 10)
		RETURNING id, enabled, interval_minutes, timeout_seconds, retention_hours, slow_threshold_ms,
		          webhook_enabled, webhook_url, webhook_debounce_count, webhook_cooldown_minutes,
		          created_at, updated_at
	`)

	cfg := &service.HealthProbeConfig{}
	if err := row.Scan(
		&cfg.ID, &cfg.Enabled, &cfg.IntervalMinutes, &cfg.TimeoutSeconds,
		&cfg.RetentionHours, &cfg.SlowThresholdMs,
		&cfg.WebhookEnabled, &cfg.WebhookURL, &cfg.WebhookDebounceCount, &cfg.WebhookCooldownMinutes,
		&cfg.CreatedAt, &cfg.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (r *healthProbeConfigRepository) UpdateConfig(ctx context.Context, cfg *service.HealthProbeConfig) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE health_probe_configs
		SET enabled = $1, interval_minutes = $2, timeout_seconds = $3, retention_hours = $4,
		    slow_threshold_ms = $5, webhook_enabled = $6, webhook_url = $7,
		    webhook_debounce_count = $8, webhook_cooldown_minutes = $9, updated_at = NOW()
		WHERE id = $10
	`, cfg.Enabled, cfg.IntervalMinutes, cfg.TimeoutSeconds, cfg.RetentionHours,
		cfg.SlowThresholdMs, cfg.WebhookEnabled, cfg.WebhookURL,
		cfg.WebhookDebounceCount, cfg.WebhookCooldownMinutes, cfg.ID)
	return err
}

// --- Result Repository ---

type healthProbeResultRepository struct {
	db *sql.DB
}

func NewHealthProbeResultRepository(db *sql.DB) service.HealthProbeResultRepository {
	return &healthProbeResultRepository{db: db}
}

func (r *healthProbeResultRepository) EnsureTable(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS health_probe_results (
			id BIGSERIAL PRIMARY KEY,
			account_id BIGINT NOT NULL,
			group_id BIGINT NOT NULL,
			probe_model TEXT NOT NULL DEFAULT '',
			status INTEGER NOT NULL DEFAULT 0,
			latency_ms INTEGER NOT NULL DEFAULT 0,
			error_type TEXT NOT NULL DEFAULT '',
			http_status_code INTEGER NOT NULL DEFAULT 0,
			error_message TEXT NOT NULL DEFAULT '',
			checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_health_probe_results_group_checked ON health_probe_results(group_id, checked_at);
		CREATE INDEX IF NOT EXISTS idx_health_probe_results_account_checked ON health_probe_results(account_id, checked_at);
		CREATE INDEX IF NOT EXISTS idx_health_probe_results_checked ON health_probe_results(checked_at)
	`)
	return err
}

func (r *healthProbeResultRepository) Create(ctx context.Context, result *service.HealthProbeResult) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO health_probe_results (account_id, group_id, probe_model, status, latency_ms, error_type, http_status_code, error_message, checked_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, result.AccountID, result.GroupID, result.ProbeModel, result.Status, result.LatencyMs,
		result.ErrorType, result.HttpStatusCode, result.ErrorMessage, result.CheckedAt)
	return err
}

func (r *healthProbeResultRepository) ListByGroup(ctx context.Context, groupID int64, since time.Time, limit int) ([]*service.HealthProbeResult, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, account_id, group_id, probe_model, status, latency_ms, error_type, http_status_code, error_message, checked_at
		FROM health_probe_results
		WHERE group_id = $1 AND checked_at >= $2
		ORDER BY checked_at DESC
		LIMIT $3
	`, groupID, since, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanProbeResults(rows)
}

func (r *healthProbeResultRepository) ListLatestByGroups(ctx context.Context) ([]*service.HealthProbeResult, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT ON (hpr.group_id)
		       hpr.id, hpr.account_id, hpr.group_id, hpr.probe_model, hpr.status, hpr.latency_ms, hpr.error_type, hpr.http_status_code, hpr.error_message, hpr.checked_at
		FROM health_probe_results hpr
		JOIN groups g ON g.id = hpr.group_id
		WHERE g.deleted_at IS NULL AND g.status = 'active'
		ORDER BY hpr.group_id, hpr.checked_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanProbeResults(rows)
}

func (r *healthProbeResultRepository) Prune(ctx context.Context, before time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM health_probe_results WHERE checked_at < $1
	`, before)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// --- Summary Repository ---

type healthProbeSummaryRepository struct {
	db *sql.DB
}

func NewHealthProbeSummaryRepository(db *sql.DB) service.HealthProbeSummaryRepository {
	return &healthProbeSummaryRepository{db: db}
}

func (r *healthProbeSummaryRepository) EnsureTable(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS health_probe_summaries (
			id BIGSERIAL PRIMARY KEY,
			group_id BIGINT NOT NULL,
			bucket_time TIMESTAMPTZ NOT NULL,
			total_probes INTEGER NOT NULL DEFAULT 0,
			success_count INTEGER NOT NULL DEFAULT 0,
			avg_latency_ms INTEGER NOT NULL DEFAULT 0,
			availability_pct REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_health_probe_summaries_group_bucket ON health_probe_summaries(group_id, bucket_time)
	`)
	return err
}

func (r *healthProbeSummaryRepository) Upsert(ctx context.Context, summary *service.HealthProbeSummary) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO health_probe_summaries (group_id, bucket_time, total_probes, success_count, avg_latency_ms, availability_pct)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (group_id, bucket_time)
		DO UPDATE SET total_probes = $3, success_count = $4, avg_latency_ms = $5, availability_pct = $6, created_at = NOW()
	`, summary.GroupID, summary.BucketTime, summary.TotalProbes, summary.SuccessCount, summary.AvgLatencyMs, summary.AvailabilityPct)
	return err
}

func (r *healthProbeSummaryRepository) ListByGroup(ctx context.Context, groupID int64, since time.Time) ([]*service.HealthProbeSummary, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, group_id, bucket_time, total_probes, success_count, avg_latency_ms, availability_pct, created_at
		FROM health_probe_summaries
		WHERE group_id = $1 AND bucket_time >= $2
		ORDER BY bucket_time ASC
	`, groupID, since)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanProbeSummaries(rows)
}

func (r *healthProbeSummaryRepository) ListAllGroups(ctx context.Context, since time.Time) ([]*service.HealthProbeSummary, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT hps.id, hps.group_id, hps.bucket_time, hps.total_probes, hps.success_count, hps.avg_latency_ms, hps.availability_pct, hps.created_at
		FROM health_probe_summaries hps
		JOIN groups g ON g.id = hps.group_id
		WHERE hps.bucket_time >= $1 AND g.deleted_at IS NULL AND g.status = 'active'
		ORDER BY hps.group_id ASC, hps.bucket_time ASC
	`, since)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanProbeSummaries(rows)
}

// --- scan helpers (reuses scannable interface from scheduled_test_repo.go) ---

func scanProbeResult(row scannable) (*service.HealthProbeResult, error) {
	r := &service.HealthProbeResult{}
	if err := row.Scan(
		&r.ID, &r.AccountID, &r.GroupID, &r.ProbeModel, &r.Status,
		&r.LatencyMs, &r.ErrorType, &r.HttpStatusCode, &r.ErrorMessage, &r.CheckedAt,
	); err != nil {
		return nil, err
	}
	return r, nil
}

func scanProbeResults(rows *sql.Rows) ([]*service.HealthProbeResult, error) {
	var results []*service.HealthProbeResult
	for rows.Next() {
		r, err := scanProbeResult(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func scanProbeSummary(row scannable) (*service.HealthProbeSummary, error) {
	s := &service.HealthProbeSummary{}
	if err := row.Scan(
		&s.ID, &s.GroupID, &s.BucketTime, &s.TotalProbes,
		&s.SuccessCount, &s.AvgLatencyMs, &s.AvailabilityPct, &s.CreatedAt,
	); err != nil {
		return nil, err
	}
	return s, nil
}

func scanProbeSummaries(rows *sql.Rows) ([]*service.HealthProbeSummary, error) {
	var summaries []*service.HealthProbeSummary
	for rows.Next() {
		s, err := scanProbeSummary(rows)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

// --- Group Config Repository ---

type healthProbeGroupConfigRepository struct {
	db *sql.DB
}

func NewHealthProbeGroupConfigRepository(db *sql.DB) service.HealthProbeGroupConfigRepository {
	return &healthProbeGroupConfigRepository{db: db}
}

func (r *healthProbeGroupConfigRepository) EnsureTable(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS health_probe_group_configs (
			id BIGSERIAL PRIMARY KEY,
			group_id BIGINT NOT NULL UNIQUE,
			probe_model TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_health_probe_group_configs_group ON health_probe_group_configs(group_id)
	`)
	return err
}

func (r *healthProbeGroupConfigRepository) Get(ctx context.Context, groupID int64) (*service.HealthProbeGroupConfig, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, group_id, probe_model, created_at, updated_at
		FROM health_probe_group_configs
		WHERE group_id = $1
	`, groupID)

	cfg := &service.HealthProbeGroupConfig{}
	err := row.Scan(&cfg.ID, &cfg.GroupID, &cfg.ProbeModel, &cfg.CreatedAt, &cfg.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func (r *healthProbeGroupConfigRepository) ListAll(ctx context.Context) ([]*service.HealthProbeGroupConfig, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT hpc.id, hpc.group_id, hpc.probe_model, hpc.created_at, hpc.updated_at
		FROM health_probe_group_configs hpc
		JOIN groups g ON g.id = hpc.group_id
		WHERE g.deleted_at IS NULL AND g.status = 'active'
		ORDER BY hpc.group_id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var configs []*service.HealthProbeGroupConfig
	for rows.Next() {
		cfg := &service.HealthProbeGroupConfig{}
		if err := rows.Scan(&cfg.ID, &cfg.GroupID, &cfg.ProbeModel, &cfg.CreatedAt, &cfg.UpdatedAt); err != nil {
			return nil, err
		}
		configs = append(configs, cfg)
	}
	return configs, rows.Err()
}

func (r *healthProbeGroupConfigRepository) Upsert(ctx context.Context, cfg *service.HealthProbeGroupConfig) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO health_probe_group_configs (group_id, probe_model)
		VALUES ($1, $2)
		ON CONFLICT (group_id)
		DO UPDATE SET probe_model = $2, updated_at = NOW()
	`, cfg.GroupID, cfg.ProbeModel)
	return err
}

func (r *healthProbeGroupConfigRepository) Delete(ctx context.Context, groupID int64) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM health_probe_group_configs WHERE group_id = $1
	`, groupID)
	return err
}
