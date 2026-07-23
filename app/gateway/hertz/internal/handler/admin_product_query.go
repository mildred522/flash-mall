package handler

import (
	"context"

	"flash-mall/app/gateway/hertz/internal/svc"
)

func loadAdminProducts(ctx context.Context, svcCtx *svc.ServiceContext, query productListQuery) ([]AdminProductItem, int64, error) {
	service, err := catalogQueryService(svcCtx)
	if err != nil {
		return nil, 0, err
	}
	return service.AdminProducts(ctx, query)
}

func loadAdminProductDetail(ctx context.Context, svcCtx *svc.ServiceContext, productID int64) (AdminProductItem, error) {
	service, err := catalogQueryService(svcCtx)
	if err != nil {
		return AdminProductItem{}, err
	}
	return service.AdminProductDetail(ctx, productID)
}
