package cache

import (
	"context"
	"encoding/json"
	"time"

	"flash-mall/app/order/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const catalogCacheKey = "catalog:cards:v1"
const catalogCacheTTL = 30 * time.Second

type CatalogCache struct {
	rds *redis.Redis
}

func NewCatalogCache(rds *redis.Redis) *CatalogCache {
	return &CatalogCache{rds: rds}
}

func (c *CatalogCache) Get(ctx context.Context) []types.ProductCard {
	val, err := c.rds.GetCtx(ctx, catalogCacheKey)
	if err != nil || val == "" {
		return nil
	}
	var items []types.ProductCard
	if err := json.Unmarshal([]byte(val), &items); err != nil {
		logx.WithContext(ctx).Errorf("catalog cache unmarshal failed: %v", err)
		return nil
	}
	return items
}

func (c *CatalogCache) Set(ctx context.Context, items []types.ProductCard) {
	data, err := json.Marshal(items)
	if err != nil {
		return
	}
	if err := c.rds.SetexCtx(ctx, catalogCacheKey, string(data), int(catalogCacheTTL.Seconds())); err != nil {
		logx.WithContext(ctx).Errorf("catalog cache set failed: %v", err)
	}
}

func (c *CatalogCache) Invalidate(ctx context.Context) {
	if _, err := c.rds.DelCtx(ctx, catalogCacheKey); err != nil {
		logx.WithContext(ctx).Errorf("catalog cache invalidate failed: %v", err)
	}
}
