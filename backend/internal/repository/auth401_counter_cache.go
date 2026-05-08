package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const auth401CounterPrefix = "auth401_count:account:"

var auth401CounterIncrScript = redis.NewScript(`
	local key = KEYS[1]
	local ttl = tonumber(ARGV[1])

	local count = redis.call('INCR', key)
	if count == 1 then
		redis.call('EXPIRE', key, ttl)
	end

	return count
`)

type auth401CounterCache struct {
	rdb *redis.Client
}

func NewAuth401CounterCache(rdb *redis.Client) service.Auth401CounterCache {
	return &auth401CounterCache{rdb: rdb}
}

func (c *auth401CounterCache) IncrementAuth401Count(ctx context.Context, accountID int64, windowMinutes int) (int64, error) {
	key := fmt.Sprintf("%s%d", auth401CounterPrefix, accountID)
	ttlSeconds := windowMinutes * 60
	if ttlSeconds < 60 {
		ttlSeconds = 60
	}
	result, err := auth401CounterIncrScript.Run(ctx, c.rdb, []string{key}, ttlSeconds).Int64()
	if err != nil {
		return 0, fmt.Errorf("increment auth401 count: %w", err)
	}
	return result, nil
}

func (c *auth401CounterCache) ResetAuth401Count(ctx context.Context, accountID int64) error {
	key := fmt.Sprintf("%s%d", auth401CounterPrefix, accountID)
	return c.rdb.Del(ctx, key).Err()
}

func (c *auth401CounterCache) GetAuth401CountTTL(ctx context.Context, accountID int64) (time.Duration, error) {
	key := fmt.Sprintf("%s%d", auth401CounterPrefix, accountID)
	return c.rdb.TTL(ctx, key).Result()
}
