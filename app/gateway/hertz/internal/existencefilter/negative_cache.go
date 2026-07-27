package existencefilter

import (
	"context"
	"fmt"
	"hash/fnv"
	"time"

	redis "github.com/redis/go-redis/v9"
)

type RedisNegativeCache struct {
	config normalizedConfig
	client *redis.Client
}

func NewRedisNegativeCache(config Config, client *redis.Client) *RedisNegativeCache {
	return &RedisNegativeCache{config: config.normalize(), client: client}
}

func (c *RedisNegativeCache) Contains(ctx context.Context, productID int64) (hit bool, err error) {
	defer func() {
		result := "miss"
		if err != nil {
			result = "error"
		} else if hit {
			result = "hit"
		}
		negativeRequests.WithLabelValues("product", result).Inc()
	}()
	if c.client == nil || productID <= 0 {
		return false, nil
	}
	opCtx, cancel := context.WithTimeout(ctx, c.config.OperationTimeout)
	defer cancel()
	exists, err := c.client.Exists(opCtx, c.key(productID)).Result()
	return exists > 0, err
}

func (c *RedisNegativeCache) Mark(ctx context.Context, productID int64) error {
	if c.client == nil || productID <= 0 {
		return nil
	}
	opCtx, cancel := context.WithTimeout(ctx, c.config.OperationTimeout)
	defer cancel()
	return c.client.Set(opCtx, c.key(productID), `{"reason":"not_public","v":1}`, c.ttl(productID)).Err()
}

func (c *RedisNegativeCache) Invalidate(ctx context.Context, productID int64) error {
	if c.client == nil || productID <= 0 {
		return nil
	}
	opCtx, cancel := context.WithTimeout(ctx, c.config.OperationTimeout)
	defer cancel()
	return c.client.Del(opCtx, c.key(productID)).Err()
}

func (c *RedisNegativeCache) key(productID int64) string {
	return fmt.Sprintf("%s:product:%d", c.config.NegativePrefix, productID)
}

func (c *RedisNegativeCache) ttl(productID int64) time.Duration {
	maxJitter := c.config.NegativeTTL / 3
	if maxJitter <= 0 {
		return c.config.NegativeTTL
	}
	hash := fnv.New32a()
	_, _ = fmt.Fprintf(hash, "%d", productID)
	jitter := time.Duration(uint64(maxJitter) * uint64(hash.Sum32()%100) / 100)
	return c.config.NegativeTTL + jitter
}
