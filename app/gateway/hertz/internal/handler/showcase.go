package handler

import (
	"context"
	"database/sql"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/svc"
	"flash-mall/app/product/rpc/productclient"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

const currentShowcaseID int64 = 1

type showcaseSlotState struct {
	ProductExists  bool
	ProductStatus  int64
	MerchantExists bool
	MerchantStatus int64
	StockAvailable int64
}

func deriveShowcaseInvalidReason(state showcaseSlotState) string {
	switch {
	case !state.ProductExists:
		return "product_not_found"
	case state.ProductStatus != 1:
		return "product_inactive"
	case !state.MerchantExists:
		return "merchant_not_found"
	case state.MerchantStatus != 1:
		return "merchant_inactive"
	case state.StockAvailable <= 0:
		return "out_of_stock"
	default:
		return ""
	}
}

func loadShowcaseLayout(ctx context.Context, db *sql.DB) (ShowcaseResp, error) {
	rows, err := db.QueryContext(ctx, `
SELECT showcase.version, showcase.operator_id,
       COALESCE(DATE_FORMAT(showcase.publish_time, '%Y-%m-%d %H:%i:%s'), ''),
       COALESCE(item.slot_no, 0), COALESCE(item.product_id, 0),
       IF(product.id IS NULL, 0, 1), COALESCE(product.status, 0),
       COALESCE(product.merchant_id, 0), IF(merchant.id IS NULL, 0, 1),
       COALESCE(merchant.status, 0), COALESCE(stock.available, product.stock, 0)
FROM mall_product.homepage_showcase showcase
LEFT JOIN mall_product.homepage_showcase_item item ON item.showcase_id = showcase.id
LEFT JOIN mall_product.product product ON product.id = item.product_id
LEFT JOIN mall_order.merchant merchant ON merchant.id = product.merchant_id
LEFT JOIN mall_product.product_stock_snapshot stock ON stock.product_id = product.id
WHERE showcase.id = ?
ORDER BY item.slot_no ASC`, currentShowcaseID)
	if err != nil {
		return ShowcaseResp{}, err
	}
	defer func() { _ = rows.Close() }()

	result := ShowcaseResp{Items: make([]ShowcaseSlot, 12)}
	for index := range result.Items {
		result.Items[index] = ShowcaseSlot{SlotNo: int64(index + 1), Empty: true}
	}
	found := false
	for rows.Next() {
		found = true
		var slotNo, productID, productExists, productStatus int64
		var merchantID, merchantExists, merchantStatus, stockAvailable int64
		if err := rows.Scan(&result.Version, &result.OperatorID, &result.PublishTime,
			&slotNo, &productID, &productExists, &productStatus, &merchantID,
			&merchantExists, &merchantStatus, &stockAvailable); err != nil {
			return ShowcaseResp{}, err
		}
		if slotNo < 1 || slotNo > 12 || productID <= 0 {
			continue
		}
		state := showcaseSlotState{
			ProductExists: productExists == 1, ProductStatus: productStatus,
			MerchantExists: merchantExists == 1, MerchantStatus: merchantStatus,
			StockAvailable: stockAvailable,
		}
		reason := deriveShowcaseInvalidReason(state)
		result.Items[slotNo-1] = ShowcaseSlot{
			SlotNo: slotNo, ProductID: productID, Valid: reason == "", InvalidReason: reason,
		}
	}
	if err := rows.Err(); err != nil {
		return ShowcaseResp{}, err
	}
	if !found {
		return ShowcaseResp{}, sql.ErrNoRows
	}
	return result, nil
}

func buildPublicShowcaseCatalog(layout ShowcaseResp, cards map[int64]ProductCard) ProductListResp {
	items := make([]ProductCard, 0, 12)
	for _, slot := range layout.Items {
		if !slot.Valid {
			continue
		}
		card, ok := cards[slot.ProductID]
		if !ok {
			continue
		}
		card.SlotNo = slot.SlotNo
		items = append(items, card)
	}
	return ProductListResp{Items: items, Total: int64(len(items)), Page: 1, PageSize: 12}
}

func ShowcaseCatalogHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "product datasource unavailable", err))
			return
		}
		if err = ensureStorefrontSchema(ctx, db); err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "showcase schema unavailable", err))
			return
		}
		layout, err := loadShowcaseLayout(ctx, db)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "showcase query failed", err))
			return
		}
		productIDs := make([]int64, 0, 12)
		for _, slot := range layout.Items {
			if slot.Valid {
				productIDs = append(productIDs, slot.ProductID)
			}
		}
		if len(productIDs) == 0 {
			ok(ctx, c, buildPublicShowcaseCatalog(layout, map[int64]ProductCard{}))
			return
		}
		resp, err := svcCtx.ProductRpc.ListProducts(ctx, &productclient.ListProductsReq{ProductIds: productIDs})
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "product service unavailable", err))
			return
		}
		cards := buildProductCards(resp.Items, loadProductMeta(ctx, svcCtx, productIDs), nil)
		ok(ctx, c, buildPublicShowcaseCatalog(layout, cards))
	}
}
