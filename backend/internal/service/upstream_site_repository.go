package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

var (
	ErrUpstreamSiteNotFound = infraerrors.NotFound("UPSTREAM_SITE_NOT_FOUND", "upstream site not found")
	ErrUpstreamSiteExists   = infraerrors.Conflict("UPSTREAM_SITE_EXISTS", "upstream site with this base_url already exists")
)

// UpstreamSiteRepository 上游站点数据访问接口
type UpstreamSiteRepository interface {
	Create(ctx context.Context, site *UpstreamSite) error
	GetByID(ctx context.Context, id int64) (*UpstreamSite, error)
	Update(ctx context.Context, site *UpstreamSite) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, params pagination.PaginationParams, status, search string) ([]UpstreamSite, *pagination.PaginationResult, error)
	ListDueForSync(ctx context.Context) ([]UpstreamSite, error)
	UpdateSyncStatus(ctx context.Context, id int64, status, syncError string, modelCount int) error
	UpdateTokenCache(ctx context.Context, id int64, accessToken, refreshToken string, expiresAt *time.Time) error
	ClearTokenCache(ctx context.Context, id int64) error
	ExistsByBaseURL(ctx context.Context, baseURL string) (bool, error)
	ExistsByBaseURLExcluding(ctx context.Context, baseURL string, excludeID int64) (bool, error)
}
