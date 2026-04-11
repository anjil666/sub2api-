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
	encrypted, err := r.encryptor.Encrypt(site.APIKey)
	if err != nil {
		return fmt.Errorf("encrypt api key: %w", err)
	}

	err = r.db.QueryRowContext(ctx,
		`INSERT INTO upstream_sites (name, platform, base_url, api_key_encrypted, price_multiplier,
			sync_enabled, sync_interval_minutes, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, created_at, updated_at`,
		site.Name, site.Platform, site.BaseURL, encrypted, site.PriceMultiplier,
		site.SyncEnabled, site.SyncIntervalMinutes, site.Status,
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
	var encrypted string
	var lastSyncAt sql.NullTime
	var managedGroupID, managedAccountID, managedChannelID sql.NullInt64

	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, platform, base_url, api_key_encrypted, price_multiplier,
			sync_enabled, sync_interval_minutes, last_sync_at, last_sync_status,
			last_sync_error, last_sync_model_count, status,
			managed_group_id, managed_account_id, managed_channel_id,
			created_at, updated_at
		 FROM upstream_sites WHERE id = $1`, id,
	).Scan(
		&site.ID, &site.Name, &site.Platform, &site.BaseURL, &encrypted, &site.PriceMultiplier,
		&site.SyncEnabled, &site.SyncIntervalMinutes, &lastSyncAt, &site.LastSyncStatus,
		&site.LastSyncError, &site.LastSyncModelCount, &site.Status,
		&managedGroupID, &managedAccountID, &managedChannelID,
		&site.CreatedAt, &site.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, service.ErrUpstreamSiteNotFound
		}
		return nil, fmt.Errorf("get upstream site: %w", err)
	}

	decrypted, err := r.encryptor.Decrypt(encrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt api key: %w", err)
	}
	site.APIKey = decrypted

	if lastSyncAt.Valid {
		site.LastSyncAt = &lastSyncAt.Time
	}
	if managedGroupID.Valid {
		site.ManagedGroupID = &managedGroupID.Int64
	}
	if managedAccountID.Valid {
		site.ManagedAccountID = &managedAccountID.Int64
	}
	if managedChannelID.Valid {
		site.ManagedChannelID = &managedChannelID.Int64
	}
	return site, nil
}

func (r *upstreamSiteRepo) Update(ctx context.Context, site *service.UpstreamSite) error {
	var encrypted string
	if site.APIKey != "" {
		var err error
		encrypted, err = r.encryptor.Encrypt(site.APIKey)
		if err != nil {
			return fmt.Errorf("encrypt api key: %w", err)
		}
	}

	var query string
	var args []any
	if encrypted != "" {
		query = `UPDATE upstream_sites
			SET name=$1, base_url=$2, api_key_encrypted=$3, price_multiplier=$4,
				sync_enabled=$5, sync_interval_minutes=$6, status=$7, updated_at=NOW()
			WHERE id=$8 RETURNING updated_at`
		args = []any{site.Name, site.BaseURL, encrypted, site.PriceMultiplier,
			site.SyncEnabled, site.SyncIntervalMinutes, site.Status, site.ID}
	} else {
		query = `UPDATE upstream_sites
			SET name=$1, base_url=$2, price_multiplier=$3,
				sync_enabled=$4, sync_interval_minutes=$5, status=$6, updated_at=NOW()
			WHERE id=$7 RETURNING updated_at`
		args = []any{site.Name, site.BaseURL, site.PriceMultiplier,
			site.SyncEnabled, site.SyncIntervalMinutes, site.Status, site.ID}
	}

	err := r.db.QueryRowContext(ctx, query, args...).Scan(&site.UpdatedAt)
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
		where = append(where, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, status)
		argIdx++
	}
	if search != "" {
		where = append(where, fmt.Sprintf("(name ILIKE $%d OR base_url ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+escapeLike(search)+"%")
		argIdx++
	}

	whereClause := strings.Join(where, " AND ")

	var total int64
	err := r.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM upstream_sites WHERE %s", whereClause), args...,
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
		`SELECT id, name, platform, base_url, api_key_encrypted, price_multiplier,
			sync_enabled, sync_interval_minutes, last_sync_at, last_sync_status,
			last_sync_error, last_sync_model_count, status,
			managed_group_id, managed_account_id, managed_channel_id,
			created_at, updated_at
		 FROM upstream_sites WHERE %s ORDER BY id DESC LIMIT $%d OFFSET $%d`,
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
		var encrypted string
		var lastSyncAt sql.NullTime
		var managedGroupID, managedAccountID, managedChannelID sql.NullInt64

		if err := rows.Scan(
			&site.ID, &site.Name, &site.Platform, &site.BaseURL, &encrypted, &site.PriceMultiplier,
			&site.SyncEnabled, &site.SyncIntervalMinutes, &lastSyncAt, &site.LastSyncStatus,
			&site.LastSyncError, &site.LastSyncModelCount, &site.Status,
			&managedGroupID, &managedAccountID, &managedChannelID,
			&site.CreatedAt, &site.UpdatedAt,
		); err != nil {
			return nil, nil, fmt.Errorf("scan upstream site: %w", err)
		}

		// 列表不解密 API Key，只用于展示
		site.APIKey = ""
		if lastSyncAt.Valid {
			site.LastSyncAt = &lastSyncAt.Time
		}
		if managedGroupID.Valid {
			site.ManagedGroupID = &managedGroupID.Int64
		}
		if managedAccountID.Valid {
			site.ManagedAccountID = &managedAccountID.Int64
		}
		if managedChannelID.Valid {
			site.ManagedChannelID = &managedChannelID.Int64
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
		`SELECT id, name, platform, base_url, api_key_encrypted, price_multiplier,
			sync_enabled, sync_interval_minutes, last_sync_at, last_sync_status,
			last_sync_error, last_sync_model_count, status,
			managed_group_id, managed_account_id, managed_channel_id,
			created_at, updated_at
		 FROM upstream_sites
		 WHERE sync_enabled = true AND status = 'active'
		   AND (last_sync_at IS NULL OR last_sync_at + (sync_interval_minutes || ' minutes')::INTERVAL < NOW())
		 ORDER BY last_sync_at ASC NULLS FIRST`)
	if err != nil {
		return nil, fmt.Errorf("query due upstream sites: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var sites []service.UpstreamSite
	for rows.Next() {
		var site service.UpstreamSite
		var encrypted string
		var lastSyncAt sql.NullTime
		var managedGroupID, managedAccountID, managedChannelID sql.NullInt64

		if err := rows.Scan(
			&site.ID, &site.Name, &site.Platform, &site.BaseURL, &encrypted, &site.PriceMultiplier,
			&site.SyncEnabled, &site.SyncIntervalMinutes, &lastSyncAt, &site.LastSyncStatus,
			&site.LastSyncError, &site.LastSyncModelCount, &site.Status,
			&managedGroupID, &managedAccountID, &managedChannelID,
			&site.CreatedAt, &site.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan upstream site: %w", err)
		}

		decrypted, err := r.encryptor.Decrypt(encrypted)
		if err != nil {
			return nil, fmt.Errorf("decrypt api key for site %d: %w", site.ID, err)
		}
		site.APIKey = decrypted

		if lastSyncAt.Valid {
			site.LastSyncAt = &lastSyncAt.Time
		}
		if managedGroupID.Valid {
			site.ManagedGroupID = &managedGroupID.Int64
		}
		if managedAccountID.Valid {
			site.ManagedAccountID = &managedAccountID.Int64
		}
		if managedChannelID.Valid {
			site.ManagedChannelID = &managedChannelID.Int64
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

func (r *upstreamSiteRepo) UpdateManagedResources(ctx context.Context, id int64, groupID, accountID, channelID *int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE upstream_sites SET managed_group_id=$1, managed_account_id=$2, managed_channel_id=$3,
			updated_at=NOW()
		 WHERE id=$4`,
		toNullInt64(groupID), toNullInt64(accountID), toNullInt64(channelID), id,
	)
	if err != nil {
		return fmt.Errorf("update managed resources: %w", err)
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
