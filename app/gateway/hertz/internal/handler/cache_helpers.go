package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	gatewaycache "flash-mall/app/gateway/hertz/internal/cache"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	showcaseCatalogCacheKey = "showcase:catalog:v1"
)

func loadCachedJSON[T any](ctx context.Context, svcCtx *svc.ServiceContext, key string, loader func(context.Context) (T, error)) (T, gatewaycache.Source, error) {
	if svcCtx.Cache == nil {
		value, err := loader(ctx)
		return value, gatewaycache.SourceOrigin, err
	}
	raw, source, err := svcCtx.Cache.GetOrLoad(ctx, key, func(loadCtx context.Context) ([]byte, error) {
		value, loadErr := loader(loadCtx)
		if loadErr != nil {
			return nil, loadErr
		}
		return json.Marshal(value)
	})
	var value T
	if err != nil {
		return value, source, err
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return value, source, err
	}
	return value, source, nil
}

func productDetailCacheKey(productID int64) string {
	return fmt.Sprintf("product:detail:%d:v1", productID)
}

func storeDetailCacheKey(merchantID int64) string {
	return fmt.Sprintf("store:detail:%d:v1", merchantID)
}

func storeProductsCacheKey(merchantID, page, pageSize int64, keyword string) string {
	digest := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(keyword))))
	return fmt.Sprintf("store:products:%d:%d:%d:%s:v1", merchantID, page, pageSize, hex.EncodeToString(digest[:6]))
}

func showcaseCandidatesCacheKey(page, pageSize, merchantID int64, keyword string) string {
	digest := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(keyword))))
	return fmt.Sprintf("showcase:candidates:%d:%d:%d:%s:v1", page, pageSize, merchantID, hex.EncodeToString(digest[:6]))
}

func invalidateStoreReadCaches(ctx context.Context, svcCtx *svc.ServiceContext, merchantID int64) {
	if svcCtx.Cache == nil || merchantID <= 0 {
		return
	}
	if err := svcCtx.Cache.Invalidate(ctx, showcaseCatalogCacheKey, storeDetailCacheKey(merchantID)); err != nil {
		logx.WithContext(ctx).Errorf("gateway cache invalidate store keys failed: merchant_id=%d err=%v", merchantID, err)
	}
	if err := svcCtx.Cache.InvalidatePrefix(ctx, fmt.Sprintf("store:products:%d:", merchantID)); err != nil {
		logx.WithContext(ctx).Errorf("gateway cache invalidate store products failed: merchant_id=%d err=%v", merchantID, err)
	}
	if err := svcCtx.Cache.InvalidatePrefix(ctx, "showcase:candidates:"); err != nil {
		logx.WithContext(ctx).Errorf("gateway cache invalidate showcase candidates failed: merchant_id=%d err=%v", merchantID, err)
	}
}

func invalidateProductReadCaches(ctx context.Context, svcCtx *svc.ServiceContext, productID, merchantID int64) {
	if svcCtx.Cache == nil || productID <= 0 {
		return
	}
	if merchantID <= 0 {
		if db, err := svcCtx.SqlConn.RawDB(); err == nil {
			_ = db.QueryRowContext(ctx, "SELECT merchant_id FROM mall_product.product WHERE id = ?", productID).Scan(&merchantID)
		}
	}
	if err := svcCtx.Cache.Invalidate(ctx, showcaseCatalogCacheKey, productDetailCacheKey(productID)); err != nil {
		logx.WithContext(ctx).Errorf("gateway cache invalidate product keys failed: product_id=%d err=%v", productID, err)
	}
	// Product detail responses contain same-store recommendations. A product
	// mutation can therefore stale detail pages other than its own.
	if err := svcCtx.Cache.InvalidatePrefix(ctx, "product:detail:"); err != nil {
		logx.WithContext(ctx).Errorf("gateway cache invalidate related product details failed: product_id=%d err=%v", productID, err)
	}
	if merchantID > 0 {
		invalidateStoreReadCaches(ctx, svcCtx, merchantID)
	}
}
