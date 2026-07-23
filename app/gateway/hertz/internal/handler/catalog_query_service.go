package handler

import (
	"context"
	"errors"

	"flash-mall/app/gateway/hertz/internal/application/catalogquery"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type productMeta = catalogquery.ProductMeta

func catalogQueryService(svcCtx *svc.ServiceContext) (*catalogquery.Service, error) {
	if svcCtx.CatalogQueries != nil {
		return svcCtx.CatalogQueries, nil
	}
	return nil, errors.New("catalog query service unavailable")
}

func loadProductMeta(ctx context.Context, svcCtx *svc.ServiceContext, productIDs []int64) map[int64]productMeta {
	service, err := catalogQueryService(svcCtx)
	if err != nil {
		logx.WithContext(ctx).Errorf("gateway product metadata service failed: %v", err)
		return map[int64]productMeta{}
	}
	result, err := service.ProductMetadata(ctx, productIDs)
	if err != nil {
		logx.WithContext(ctx).Errorf("gateway product metadata query failed: %v", err)
		return map[int64]productMeta{}
	}
	return result
}
