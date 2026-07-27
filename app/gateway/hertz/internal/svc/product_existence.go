package svc

import (
	"flash-mall/app/gateway/hertz/internal/existencefilter"

	redis "github.com/redis/go-redis/v9"
)

func configureProductExistence(
	svcCtx *ServiceContext,
	client *redis.Client,
	source existencefilter.IDSource,
) *existencefilter.RedisBitmap {
	if !svcCtx.Config.ProductExistenceFilterEnabled || client == nil || source == nil {
		return nil
	}
	filterConfig := svcCtx.Config.ProductExistenceConfig()
	filter := existencefilter.NewRedisBitmap(filterConfig, client, source)
	svcCtx.ProductExistenceFilter = filter
	svcCtx.ProductNegativeCache = existencefilter.NewRedisNegativeCache(filterConfig, client)
	return filter
}
