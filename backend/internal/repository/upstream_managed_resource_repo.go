package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type upstreamManagedResourceRepo struct {
	db        *sql.DB
	encryptor service.SecretEncryptor
}

// NewUpstreamManagedResourceRepository 创建上游托管资源仓库
func NewUpstreamManagedResourceRepository(db *sql.DB, encryptor service.SecretEncryptor) service.UpstreamManagedResourceRepository {
	return &upstreamManagedResourceRepo{db: db, encryptor: encryptor}
}

func (r *upstreamManagedResourceRepo) Upsert(ctx context.Context, res *service.UpstreamManagedResource) error {
	apiKeyEnc, err := r.encryptor.Encrypt(res.APIKey)
	if err != nil {
		return fmt.Errorf("encrypt api key: %w", err)
	}

	err = r.db.QueryRowContext(ctx,
		`INSERT INTO upstream_managed_resources
			(upstream_site_id, upstream_key_id, upstream_key_prefix, upstream_key_name,
			 upstream_group_id, api_key_encrypted, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (upstream_site_id, upstream_key_id) DO UPDATE SET
			upstream_key_prefix = EXCLUDED.upstream_key_prefix,
			upstream_key_name = EXCLUDED.upstream_key_name,
			upstream_group_id = EXCLUDED.upstream_group_id,
			api_key_encrypted = EXCLUDED.api_key_encrypted,
			updated_at = NOW()
		 RETURNING id, status, created_at, updated_at`,
		res.UpstreamSiteID, res.UpstreamKeyID, res.UpstreamKeyPrefix, res.UpstreamKeyName,
		toNullInt64(res.UpstreamGroupID), apiKeyEnc, res.Status,
	).Scan(&res.ID, &res.Status, &res.CreatedAt, &res.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert managed resource: %w", err)
	}
	return nil
}

func (r *upstreamManagedResourceRepo) ListBySiteID(ctx context.Context, siteID int64) ([]*service.UpstreamManagedResource, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, upstream_site_id, upstream_key_id, upstream_key_prefix, upstream_key_name,
			upstream_group_id, api_key_encrypted,
			managed_group_id, managed_account_id, managed_channel_id,
			price_multiplier, upstream_rate_multiplier,
			model_count, status, disabled_by, last_synced_at, created_at, updated_at
		 FROM upstream_managed_resources
		 WHERE upstream_site_id = $1 ORDER BY id`, siteID,
	)
	if err != nil {
		return nil, fmt.Errorf("list managed resources: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var resources []*service.UpstreamManagedResource
	for rows.Next() {
		res := &service.UpstreamManagedResource{}
		var apiKeyEnc string
		var upstreamGroupID, managedGroupID, managedAccountID, managedChannelID sql.NullInt64
		var lastSyncedAt sql.NullTime

		if err := rows.Scan(
			&res.ID, &res.UpstreamSiteID, &res.UpstreamKeyID, &res.UpstreamKeyPrefix, &res.UpstreamKeyName,
			&upstreamGroupID, &apiKeyEnc,
			&managedGroupID, &managedAccountID, &managedChannelID,
			&res.PriceMultiplier, &res.UpstreamRateMultiplier,
			&res.ModelCount, &res.Status, &res.DisabledBy, &lastSyncedAt, &res.CreatedAt, &res.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan managed resource: %w", err)
		}

		// 解密 API Key
		if apiKeyEnc != "" {
			if res.APIKey, err = r.encryptor.Decrypt(apiKeyEnc); err != nil {
				return nil, fmt.Errorf("decrypt api key for resource %d: %w", res.ID, err)
			}
		}

		if upstreamGroupID.Valid {
			res.UpstreamGroupID = &upstreamGroupID.Int64
		}
		if managedGroupID.Valid {
			res.ManagedGroupID = &managedGroupID.Int64
		}
		if managedAccountID.Valid {
			res.ManagedAccountID = &managedAccountID.Int64
		}
		if managedChannelID.Valid {
			res.ManagedChannelID = &managedChannelID.Int64
		}
		if lastSyncedAt.Valid {
			res.LastSyncedAt = &lastSyncedAt.Time
		}
		resources = append(resources, res)
	}
	return resources, rows.Err()
}

func (r *upstreamManagedResourceRepo) GetBySiteAndKeyID(ctx context.Context, siteID int64, upstreamKeyID string) (*service.UpstreamManagedResource, error) {
	res := &service.UpstreamManagedResource{}
	var apiKeyEnc string
	var upstreamGroupID, managedGroupID, managedAccountID, managedChannelID sql.NullInt64
	var lastSyncedAt sql.NullTime

	err := r.db.QueryRowContext(ctx,
		`SELECT id, upstream_site_id, upstream_key_id, upstream_key_prefix, upstream_key_name,
			upstream_group_id, api_key_encrypted,
			managed_group_id, managed_account_id, managed_channel_id,
			price_multiplier, upstream_rate_multiplier,
			model_count, status, disabled_by, last_synced_at, created_at, updated_at
		 FROM upstream_managed_resources
		 WHERE upstream_site_id = $1 AND upstream_key_id = $2`, siteID, upstreamKeyID,
	).Scan(
		&res.ID, &res.UpstreamSiteID, &res.UpstreamKeyID, &res.UpstreamKeyPrefix, &res.UpstreamKeyName,
		&upstreamGroupID, &apiKeyEnc,
		&managedGroupID, &managedAccountID, &managedChannelID,
		&res.PriceMultiplier, &res.UpstreamRateMultiplier,
		&res.ModelCount, &res.Status, &res.DisabledBy, &lastSyncedAt, &res.CreatedAt, &res.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get managed resource: %w", err)
	}

	if apiKeyEnc != "" {
		if res.APIKey, err = r.encryptor.Decrypt(apiKeyEnc); err != nil {
			return nil, fmt.Errorf("decrypt api key: %w", err)
		}
	}
	if upstreamGroupID.Valid {
		res.UpstreamGroupID = &upstreamGroupID.Int64
	}
	if managedGroupID.Valid {
		res.ManagedGroupID = &managedGroupID.Int64
	}
	if managedAccountID.Valid {
		res.ManagedAccountID = &managedAccountID.Int64
	}
	if managedChannelID.Valid {
		res.ManagedChannelID = &managedChannelID.Int64
	}
	if lastSyncedAt.Valid {
		res.LastSyncedAt = &lastSyncedAt.Time
	}
	return res, nil
}

func (r *upstreamManagedResourceRepo) UpdateManagedIDs(ctx context.Context, id int64, groupID, accountID, channelID *int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE upstream_managed_resources
		 SET managed_group_id=$1, managed_account_id=$2, managed_channel_id=$3, updated_at=NOW()
		 WHERE id=$4`,
		toNullInt64(groupID), toNullInt64(accountID), toNullInt64(channelID), id,
	)
	if err != nil {
		return fmt.Errorf("update managed IDs: %w", err)
	}
	return nil
}

func (r *upstreamManagedResourceRepo) UpdateModelCount(ctx context.Context, id int64, count int) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE upstream_managed_resources SET model_count=$1, last_synced_at=NOW(), updated_at=NOW() WHERE id=$2`,
		count, id,
	)
	if err != nil {
		return fmt.Errorf("update model count: %w", err)
	}
	return nil
}

func (r *upstreamManagedResourceRepo) GetByID(ctx context.Context, id int64) (*service.UpstreamManagedResource, error) {
	res := &service.UpstreamManagedResource{}
	var apiKeyEnc string
	var upstreamGroupID, managedGroupID, managedAccountID, managedChannelID sql.NullInt64
	var lastSyncedAt sql.NullTime

	err := r.db.QueryRowContext(ctx,
		`SELECT id, upstream_site_id, upstream_key_id, upstream_key_prefix, upstream_key_name,
			upstream_group_id, api_key_encrypted,
			managed_group_id, managed_account_id, managed_channel_id,
			price_multiplier, upstream_rate_multiplier,
			model_count, status, disabled_by, last_synced_at, created_at, updated_at
		 FROM upstream_managed_resources WHERE id = $1`, id,
	).Scan(
		&res.ID, &res.UpstreamSiteID, &res.UpstreamKeyID, &res.UpstreamKeyPrefix, &res.UpstreamKeyName,
		&upstreamGroupID, &apiKeyEnc,
		&managedGroupID, &managedAccountID, &managedChannelID,
		&res.PriceMultiplier, &res.UpstreamRateMultiplier,
		&res.ModelCount, &res.Status, &res.DisabledBy, &lastSyncedAt, &res.CreatedAt, &res.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get managed resource: %w", err)
	}

	if apiKeyEnc != "" {
		if res.APIKey, err = r.encryptor.Decrypt(apiKeyEnc); err != nil {
			return nil, fmt.Errorf("decrypt api key: %w", err)
		}
	}
	if upstreamGroupID.Valid {
		res.UpstreamGroupID = &upstreamGroupID.Int64
	}
	if managedGroupID.Valid {
		res.ManagedGroupID = &managedGroupID.Int64
	}
	if managedAccountID.Valid {
		res.ManagedAccountID = &managedAccountID.Int64
	}
	if managedChannelID.Valid {
		res.ManagedChannelID = &managedChannelID.Int64
	}
	if lastSyncedAt.Valid {
		res.LastSyncedAt = &lastSyncedAt.Time
	}
	return res, nil
}

func (r *upstreamManagedResourceRepo) UpdatePriceMultiplier(ctx context.Context, id int64, multiplier float64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE upstream_managed_resources SET price_multiplier=$1, updated_at=NOW() WHERE id=$2`,
		multiplier, id,
	)
	if err != nil {
		return fmt.Errorf("update price multiplier: %w", err)
	}
	return nil
}

func (r *upstreamManagedResourceRepo) UpdateUpstreamRateMultiplier(ctx context.Context, id int64, multiplier float64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE upstream_managed_resources SET upstream_rate_multiplier=$1, updated_at=NOW() WHERE id=$2`,
		multiplier, id,
	)
	if err != nil {
		return fmt.Errorf("update upstream rate multiplier: %w", err)
	}
	return nil
}

func (r *upstreamManagedResourceRepo) UpdateStatus(ctx context.Context, id int64, status string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE upstream_managed_resources SET status=$1, updated_at=NOW() WHERE id=$2`,
		status, id,
	)
	if err != nil {
		return fmt.Errorf("update resource status: %w", err)
	}
	return nil
}

func (r *upstreamManagedResourceRepo) DeleteBySiteID(ctx context.Context, siteID int64) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM upstream_managed_resources WHERE upstream_site_id = $1`, siteID,
	)
	if err != nil {
		return fmt.Errorf("delete managed resources by site: %w", err)
	}
	return nil
}

func (r *upstreamManagedResourceRepo) DeleteByID(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM upstream_managed_resources WHERE id = $1`, id,
	)
	if err != nil {
		return fmt.Errorf("delete managed resource: %w", err)
	}
	return nil
}

func (r *upstreamManagedResourceRepo) DeleteStale(ctx context.Context, siteID int64, activeKeyIDs []string) (int, error) {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM upstream_managed_resources
		 WHERE upstream_site_id = $1 AND upstream_key_id != ALL($2::text[])`,
		siteID, pq.Array(activeKeyIDs),
	)
	if err != nil {
		return 0, fmt.Errorf("delete stale resources: %w", err)
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

// DisableStale 将不在 activeKeyIDs 中且当前 active 的资源标记为 disabled（自动下架）
func (r *upstreamManagedResourceRepo) DisableStale(ctx context.Context, siteID int64, activeKeyIDs []string) ([]*service.UpstreamManagedResource, error) {
	rows, err := r.db.QueryContext(ctx,
		`UPDATE upstream_managed_resources
		 SET status = 'disabled', disabled_by = 'auto', updated_at = NOW()
		 WHERE upstream_site_id = $1
		   AND upstream_key_id != ALL($2::text[])
		   AND status = 'active'
		 RETURNING id, managed_group_id, managed_account_id, managed_channel_id, upstream_key_name`,
		siteID, pq.Array(activeKeyIDs),
	)
	if err != nil {
		return nil, fmt.Errorf("disable stale resources: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var resources []*service.UpstreamManagedResource
	for rows.Next() {
		res := &service.UpstreamManagedResource{}
		var managedGroupID, managedAccountID, managedChannelID sql.NullInt64
		if err := rows.Scan(&res.ID, &managedGroupID, &managedAccountID, &managedChannelID, &res.UpstreamKeyName); err != nil {
			return nil, fmt.Errorf("scan disabled resource: %w", err)
		}
		if managedGroupID.Valid {
			res.ManagedGroupID = &managedGroupID.Int64
		}
		if managedAccountID.Valid {
			res.ManagedAccountID = &managedAccountID.Int64
		}
		if managedChannelID.Valid {
			res.ManagedChannelID = &managedChannelID.Int64
		}
		resources = append(resources, res)
	}
	return resources, rows.Err()
}

func (r *upstreamManagedResourceRepo) UpdateDisabledBy(ctx context.Context, id int64, disabledBy string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE upstream_managed_resources SET disabled_by = $1, updated_at = NOW() WHERE id = $2`,
		disabledBy, id,
	)
	if err != nil {
		return fmt.Errorf("update disabled_by: %w", err)
	}
	return nil
}

func (r *upstreamManagedResourceRepo) CountBySiteID(ctx context.Context, siteID int64) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM upstream_managed_resources WHERE upstream_site_id = $1`, siteID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count managed resources: %w", err)
	}
	return count, nil
}
