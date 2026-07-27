package handler

import (
	"context"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/existencefilter"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

func protectProductDetailOrigin(ctx context.Context, svcCtx *svc.ServiceContext, productID int64) (bool, error) {
	if svcCtx.ProductNegativeCache != nil {
		hit, err := svcCtx.ProductNegativeCache.Contains(ctx, productID)
		if err != nil {
			logx.WithContext(ctx).Errorf("product negative cache lookup failed: err=%v", err)
		} else if hit {
			return false, productNotFound()
		}
	}
	if svcCtx.ProductExistenceFilter == nil {
		return false, nil
	}
	result, err := svcCtx.ProductExistenceFilter.Check(ctx, productID)
	if err != nil {
		logx.WithContext(ctx).Errorf("product existence filter lookup failed: err=%v", err)
		return false, nil
	}
	if result == existencefilter.ResultAbsent {
		return false, productNotFound()
	}
	return result == existencefilter.ResultPossible, nil
}

func markProductNotPublic(ctx context.Context, svcCtx *svc.ServiceContext, productID int64) {
	if svcCtx.ProductNegativeCache == nil {
		return
	}
	if err := svcCtx.ProductNegativeCache.Mark(ctx, productID); err != nil {
		logx.WithContext(ctx).Errorf("product negative cache mark failed: err=%v", err)
	}
}

func prepareProductVisibility(ctx context.Context, svcCtx *svc.ServiceContext, productID int64) error {
	if svcCtx.ProductExistenceFilter == nil {
		return nil
	}
	if err := svcCtx.ProductExistenceFilter.Add(ctx, productID); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "prepare product existence filter", err)
	}
	return nil
}

func invalidateProductProtection(ctx context.Context, svcCtx *svc.ServiceContext, productID int64) {
	if svcCtx.ProductNegativeCache == nil {
		return
	}
	if err := svcCtx.ProductNegativeCache.Invalidate(ctx, productID); err != nil {
		logx.WithContext(ctx).Errorf("product negative cache invalidate failed: err=%v", err)
	}
}

func productNotFound() error {
	return apperror.New(apperror.CodeProductNotFound, "product not found")
}
