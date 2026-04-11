package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type upstreamSiteRepo struct {
	db        *sql.DB
	encryptor service.SecretEncryptor
}

// NewUpstreamSiteRepository 创建上游站点仓库
func NewUpstreamSiteRepository(db *sql.DB, encryptor service.SecretEncryptor) service.UpstreamSiteRepository {
	return &upstreamSiteRepo{db: db, encryptor: encryptor}
}

func (r *upstreamSiteRepo) Create(ctx context.Context, site *service.UpstreamSite) error {
	apiKeyEnc, err := r.encryptor.Encrypt(site.APIKey)
	if err != nil {
		return fmt.Errorf("encrypt api key: %w", err)
	}
	emailEnc, err := r.encryptor.Encrypt(site.Email)
	if err != nil {
		return fmt.Errorf("encrypt email: %w", err)
	}
	passwordEnc, err := r.encryptor.Encrypt(site.Password)
	if err != nil {
		return fmt.Errorf("encrypt password: %w", err)
	}

	err = r.db.QueryRowContext(ctx,
		`INSERT INTO upstream_sites (name, platform, base_url, api_key_encrypted,
			credential_mode, email_encrypted, password_encrypted,
			price_multiplier, sync_enabled, sync_interval_minutes, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id, created_at, updated_at`,
		site.Name, site.Platform, site.BaseURL, apiKeyEnc,
		site.CredentialMode, emailEnc, passwordEnc,
		site.PriceMultiplier, site.SyncEnabled, site.SyncIntervalMinutes, site.Status,
	).Scan(&site.ID, &site.CreatedAt, &site.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return service.ErrUpstreamSiteExists
		}
		return fmt.Errorf("insert upstream site: %w", err)
	}
	return nil
}

func (r *upstreamSiteRepo) GetByID(ctx context.Context, id int64) (*service.UpstreamSite, error) {
	site := &service.UpstreamSite{}
	var apiKeyEnc, emailEnc, passwordEnc, cachedAccessToken, cachedRefreshToken string
	var lastSyncAt sql.NullTime
	var tokenExpiresAt sql.NullTime

	err := r.db.QueryRowContext(ctx,
		`SELECT us.id, us.name, us.platform, us.base_url, us.api_key_encrypted,
			us.credential_mode, us.email_encrypted, us.password_encrypted,
			us.cached_access_token, us.cached_refresh_token, us.token_expires_at,
			us.price_multiplier, us.sync_enabled, us.sync_interval_minutes,
			us.last_sync_at, us.last_sync_status, us.last_sync_error, us.last_sync_model_count,
			us.status, us.created_at, us.updated_at,
			(SELECT COUNT(*) FROM upstream_managed_resources WHERE upstream_site_id = us.id)
		 FROM upstream_sites us WHERE us.id = $1`, id,
	).Scan(
		&site.ID, &site.Name, &site.Platform, &site.BaseURL, &apiKeyEnc,
		&site.CredentialMode, &emailEnc, &passwordEnc,
		&cachedAccessToken, &cachedRefreshToken, &tokenExpiresAt,
		&site.PriceMultiplier, &site.SyncEnabled, &site.SyncIntervalMinutes,
		&lastSyncAt, &site.LastSyncStatus, &site.LastSyncError, &site.LastSyncModelCount,
		&site.Status, &site.CreatedAt, &site.UpdatedAt,
		&site.ManagedResourceCount,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, service.ErrUpstreamSiteNotFound
		}
		return nil, fmt.Errorf("get upstream site: %w", err)
	}

	// 解密敏感字段
	if apiKeyEnc != "" {
		if site.APIKey, err = r.encryptor.Decrypt(apiKeyEnc); err != nil {
			return nil, fmt.Errorf("decrypt api key: %w", err)
		}
	}
	if emailEnc != "" {
		if site.Email, err = r.encryptor.Decrypt(emailEnc); err != nil {
			return nil, fmt.Errorf("decrypt email: %w", err)
		}
	}
	if passwordEnc != "" {
		if site.Password, err = r.encryptor.Decrypt(passwordEnc); err != nil {
			return nil, fmt.Errorf("decrypt password: %w", err)
		}
	}
	if cachedAccessToken != "" {
		if site.CachedAccessToken, err = r.encryptor.Decrypt(cachedAccessToken); err != nil {
			return nil, fmt.Errorf("decrypt access token: %w", err)
		}
	}
	if cachedRefreshToken != "" {
		if site.CachedRefreshToken, err = r.encryptor.Decrypt(cachedRefreshToken); err != nil {
			return nil, fmt.Errorf("decrypt refresh token: %w", err)
		}
	}

	if lastSyncAt.Valid {
		site.LastSyncAt = &lastSyncAt.Time
	}
	if tokenExpiresAt.Valid {
		site.TokenExpiresAt = &tokenExpiresAt.Time
	}
	return site, nil
}

func (r *upstreamSiteRepo) Update(ctx context.Context, site *service.UpstreamSite) error {
	var apiKeyEnc string
	if site.APIKey != "" {
		var err error
		apiKeyEnc, err = r.encryptor.Encrypt(site.APIKey)
		if err != nil {
			return fmt.Errorf("encrypt api key: %w", err)
		}
	}

	emailEnc, err := r.encryptor.Encrypt(site.Email)
	if err != nil {
		return fmt.Errorf("encrypt email: %w", err)
	}
	passwordEnc, err := r.encryptor.Encrypt(site.Password)
	if err != nil {
		return fmt.Errorf("encrypt password: %w", err)
	}

	var query string
	var args []any
	if apiKeyEnc != "" {
		query = `UPDATE upstream_sites
			SET name=$1, base_url=$2, api_key_encrypted=$3, credential_mode=$4,
				email_encrypted=$5, password_encrypted=$6,
				price_multiplier=$7, sync_enabled=$8, sync_interval_minutes=$9, status=$10,
				updated_at=NOW()
			WHERE id=$11 RETURNING updated_at`
		args = []any{site.Name, site.BaseURL, apiKeyEnc, site.CredentialMode,
			emailEnc, passwordEnc,
			site.PriceMultiplier, site.SyncEnabled, site.SyncIntervalMinutes, site.Status, site.ID}
	} else {
		query = `UPDATE upstream_sites
			SET name=$1, base_url=$2, credential_mode=$3,
				email_encrypted=$4, password_encrypted=$5,
				price_multiplier=$6, sync_enabled=$7, sync_interval_minutes=$8, status=$9,
				updated_at=NOW()
			WHERE id=$10 RETURNING updated_at`
		args = []any{site.Name, site.BaseURL, site.CredentialMode,
			emailEnc, passwordEnc,
			site.PriceMultiplier, site.SyncEnabled, site.SyncIntervalMinutes, site.Status, site.ID}
	}

	err = r.db.QueryRowContext(ctx, query, args...).Scan(&site.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return service.ErrUpstreamSiteNotFound
		}
		if isUniqueViolation(err) {
			return service.ErrUpstreamSiteExists
		}
		return fmt.Errorf("update upstream site: %w", err)
	}
	return nil
}

func (r *upstreamSiteRepo) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM upstream_sites WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete upstream site: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return service.ErrUpstreamSiteNotFound
	}
	return nil
}

func (r *upstreamSiteRepo) List(ctx context.Context, params pagination.PaginationParams, status, search string) ([]service.UpstreamSite, *pagination.PaginationResult, error) {
	where := []string{"1=1"}
	args := []any{}
	argIdx := 1

	if status != "" {
		where = append(where, fmt.Sprintf("us.status = $%d", argIdx))
		args = append(args, status)
		argIdx++
	}
	if search != "" {
		where = append(where, fmt.Sprintf("(us.name ILIKE $%d OR us.base_url ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+escapeLike(search)+"%")
		argIdx++
	}

	whereClause := strings.Join(where, " AND ")

	var total int64
	err := r.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM upstream_sites us WHERE %s", whereClause), args...,
	).Scan(&total)
	if err != nil {
		return nil, nil, fmt.Errorf("count upstream sites: %w", err)
	}

	pageSize := params.Limit()
	page := params.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	dataQuery := fmt.Sprintf(
		`SELECT us.id, us.name, us.platform, us.base_url, us.credential_mode,
			us.price_multiplier, us.sync_enabled, us.sync_interval_minutes,
			us.last_sync_at, us.last_sync_status, us.last_sync_error, us.last_sync_model_count,
			us.status, us.created_at, us.updated_at,
			(SELECT COUNT(*) FROM upstream_managed_resources WHERE upstream_site_id = us.id)
		 FROM upstream_sites us WHERE %s ORDER BY us.id DESC LIMIT $%d OFFSET $%d`,
		whereClause, argIdx, argIdx+1,
	)
	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("query upstream sites: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var sites []service.UpstreamSite
	for rows.Next() {
		var site service.UpstreamSite
		var lastSyncAt sql.NullTime

		if err := rows.Scan(
			&site.ID, &site.Name, &site.Platform, &site.BaseURL, &site.CredentialMode,
			&site.PriceMultiplier, &site.SyncEnabled, &site.SyncIntervalMinutes,
			&lastSyncAt, &site.LastSyncStatus, &site.LastSyncError, &site.LastSyncModelCount,
			&site.Status, &site.CreatedAt, &site.UpdatedAt,
			&site.ManagedResourceCount,
		); err != nil {
			return nil, nil, fmt.Errorf("scan upstream site: %w", err)
		}

		// 列表不解密敏感字段
		if lastSyncAt.Valid {
			site.LastSyncAt = &lastSyncAt.Time
		}
		sites = append(sites, site)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate upstream sites: %w", err)
	}

	pages := 0
	if total > 0 {
		pages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	return sites, &pagination.PaginationResult{
		Total: total, Page: page, PageSize: pageSize, Pages: pages,
	}, nil
}

func (r *upstreamSiteRepo) ListDueForSync(ctx context.Context) ([]service.UpstreamSite, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT us.id, us.name, us.platform, us.base_url, us.api_key_encrypted,
			us.credential_mode, us.email_encrypted, us.password_encrypted,
			us.cached_access_token, us.cached_refresh_token, us.token_expires_at,
			us.price_multiplier, us.sync_enabled, us.sync_interval_minutes,
			us.last_sync_at, us.last_sync_status, us.last_sync_error, us.last_sync_model_count,
			us.status, us.created_at, us.updated_at
		 FROM upstream_sites us
		 WHERE us.sync_enabled = true AND us.status = 'active'
		   AND (us.last_sync_at IS NULL OR us.last_sync_at + (us.sync_interval_minutes || ' minutes')::INTERVAL < NOW())
		 ORDER BY us.last_sync_at ASC NULLS FIRST`)
	if err != nil {
		return nil, fmt.Errorf("query due upstream sites: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var sites []service.UpstreamSite
	for rows.Next() {
		var site service.UpstreamSite
		var apiKeyEnc, emailEnc, passwordEnc, cachedAccessToken, cachedRefreshToken string
		var lastSyncAt sql.NullTime
		var tokenExpiresAt sql.NullTime

		if err := rows.Scan(
			&site.ID, &site.Name, &site.Platform, &site.BaseURL, &apiKeyEnc,
			&site.CredentialMode, &emailEnc, &passwordEnc,
			&cachedAccessToken, &cachedRefreshToken, &tokenExpiresAt,
			&site.PriceMultiplier, &site.SyncEnabled, &site.SyncIntervalMinutes,
			&lastSyncAt, &site.LastSyncStatus, &site.LastSyncError, &site.LastSyncModelCount,
			&site.Status, &site.CreatedAt, &site.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan upstream site: %w", err)
		}

		// 解密
		var decErr error
		if apiKeyEnc != "" {
			if site.APIKey, decErr = r.encryptor.Decrypt(apiKeyEnc); decErr != nil {
				return nil, fmt.Errorf("decrypt api key for site %d: %w", site.ID, decErr)
			}
		}
		if emailEnc != "" {
			if site.Email, decErr = r.encryptor.Decrypt(emailEnc); decErr != nil {
				return nil, fmt.Errorf("decrypt email for site %d: %w", site.ID, decErr)
			}
		}
		if passwordEnc != "" {
			if site.Password, decErr = r.encryptor.Decrypt(passwordEnc); decErr != nil {
				return nil, fmt.Errorf("decrypt password for site %d: %w", site.ID, decErr)
			}
		}
		if cachedAccessToken != "" {
			if site.CachedAccessToken, decErr = r.encryptor.Decrypt(cachedAccessToken); decErr != nil {
				return nil, fmt.Errorf("decrypt access token for site %d: %w", site.ID, decErr)
			}
		}
		if cachedRefreshToken != "" {
			if site.CachedRefreshToken, decErr = r.encryptor.Decrypt(cachedRefreshToken); decErr != nil {
				return nil, fmt.Errorf("decrypt refresh token for site %d: %w", site.ID, decErr)
			}
		}

		if lastSyncAt.Valid {
			site.LastSyncAt = &lastSyncAt.Time
		}
		if tokenExpiresAt.Valid {
			site.TokenExpiresAt = &tokenExpiresAt.Time
		}
		sites = append(sites, site)
	}
	return sites, rows.Err()
}

func (r *upstreamSiteRepo) UpdateSyncStatus(ctx context.Context, id int64, status, syncError string, modelCount int) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE upstream_sites SET last_sync_status=$1, last_sync_error=$2, last_sync_model_count=$3,
			last_sync_at=NOW(), updated_at=NOW()
		 WHERE id=$4`,
		status, syncError, modelCount, id,
	)
	if err != nil {
		return fmt.Errorf("update sync status: %w", err)
	}
	return nil
}

func (r *upstreamSiteRepo) UpdateTokenCache(ctx context.Context, id int64, accessToken, refreshToken string, expiresAt *time.Time) error {
	atEnc, err := r.encryptor.Encrypt(accessToken)
	if err != nil {
		return fmt.Errorf("encrypt access token: %w", err)
	}
	rtEnc, err := r.encryptor.Encrypt(refreshToken)
	if err != nil {
		return fmt.Errorf("encrypt refresh token: %w", err)
	}

	_, err = r.db.ExecContext(ctx,
		`UPDATE upstream_sites SET cached_access_token=$1, cached_refresh_token=$2, token_expires_at=$3,
			updated_at=NOW()
		 WHERE id=$4`,
		atEnc, rtEnc, expiresAt, id,
	)
	if err != nil {
		return fmt.Errorf("update token cache: %w", err)
	}
	return nil
}

func (r *upstreamSiteRepo) ClearTokenCache(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE upstream_sites SET cached_access_token='', cached_refresh_token='', token_expires_at=NULL,
			updated_at=NOW()
		 WHERE id=$1`, id,
	)
	if err != nil {
		return fmt.Errorf("clear token cache: %w", err)
	}
	return nil
}

func (r *upstreamSiteRepo) ExistsByBaseURL(ctx context.Context, baseURL string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM upstream_sites WHERE base_url = $1)`, baseURL,
	).Scan(&exists)
	return exists, err
}

func (r *upstreamSiteRepo) ExistsByBaseURLExcluding(ctx context.Context, baseURL string, excludeID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM upstream_sites WHERE base_url = $1 AND id != $2)`, baseURL, excludeID,
	).Scan(&exists)
	return exists, err
}

func toNullInt64(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}
