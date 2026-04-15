package service

import "context"

// UpstreamManagedResourceRepository 上游托管资源数据访问接口
type UpstreamManagedResourceRepository interface {
	Upsert(ctx context.Context, resource *UpstreamManagedResource) error
	ListBySiteID(ctx context.Context, siteID int64) ([]*UpstreamManagedResource, error)
	GetBySiteAndKeyID(ctx context.Context, siteID int64, upstreamKeyID string) (*UpstreamManagedResource, error)
	GetByID(ctx context.Context, id int64) (*UpstreamManagedResource, error)
	UpdateManagedIDs(ctx context.Context, id int64, groupID, accountID, channelID *int64) error
	UpdateModelCount(ctx context.Context, id int64, count int) error
	UpdatePriceMultiplier(ctx context.Context, id int64, multiplier float64) error
	UpdateUpstreamRateMultiplier(ctx context.Context, id int64, multiplier float64) error
	UpdateStatus(ctx context.Context, id int64, status string) error
	UpdateDisabledBy(ctx context.Context, id int64, disabledBy string) error
	DeleteBySiteID(ctx context.Context, siteID int64) error
	DeleteStale(ctx context.Context, siteID int64, activeKeyIDs []string) (int, error)
	DisableStale(ctx context.Context, siteID int64, activeKeyIDs []string) ([]*UpstreamManagedResource, error)
	CountBySiteID(ctx context.Context, siteID int64) (int, error)
}
