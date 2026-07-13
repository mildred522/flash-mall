package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/svc"
	"flash-mall/app/product/rpc/productclient"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

const currentShowcaseID int64 = 1

var (
	errShowcaseInvalidDraft    = errors.New("invalid showcase draft")
	errShowcaseStateConflict   = errors.New("showcase product state conflict")
	errShowcaseVersionConflict = errors.New("showcase version conflict")
)

type showcasePublishItem struct {
	SlotNo    int64 `json:"slot_no"`
	ProductID int64 `json:"product_id"`
}

type showcasePublishReq struct {
	ExpectedVersion int64                 `json:"expected_version"`
	Items           []showcasePublishItem `json:"items"`
}

type showcaseProductState struct {
	ProductID      int64
	ProductExists  bool
	ProductStatus  int64
	MerchantID     int64
	MerchantExists bool
	MerchantStatus int64
	StockAvailable int64
}

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

func validateShowcaseDraft(items []showcasePublishItem, states map[int64]showcaseProductState) error {
	if err := validateShowcaseDraftShape(items); err != nil {
		return err
	}
	merchantCounts := make(map[int64]int)
	for _, item := range items {
		state, exists := states[item.ProductID]
		if !exists {
			state = showcaseProductState{ProductID: item.ProductID}
		}
		reason := deriveShowcaseInvalidReason(showcaseSlotState{
			ProductExists: state.ProductExists, ProductStatus: state.ProductStatus,
			MerchantExists: state.MerchantExists, MerchantStatus: state.MerchantStatus,
			StockAvailable: state.StockAvailable,
		})
		if reason != "" {
			return fmt.Errorf("%w: slot=%d product=%d reason=%s", errShowcaseStateConflict, item.SlotNo, item.ProductID, reason)
		}
		merchantCounts[state.MerchantID]++
		if merchantCounts[state.MerchantID] > 2 {
			return fmt.Errorf("%w: merchant %d exceeds two slots", errShowcaseInvalidDraft, state.MerchantID)
		}
	}
	return nil
}

func validateShowcaseDraftShape(items []showcasePublishItem) error {
	if len(items) > 12 {
		return fmt.Errorf("%w: at most 12 items are allowed", errShowcaseInvalidDraft)
	}
	slots := make(map[int64]struct{}, len(items))
	products := make(map[int64]struct{}, len(items))
	for _, item := range items {
		if item.SlotNo < 1 || item.SlotNo > 12 || item.ProductID <= 0 {
			return fmt.Errorf("%w: slot and product must be positive and in range", errShowcaseInvalidDraft)
		}
		if _, exists := slots[item.SlotNo]; exists {
			return fmt.Errorf("%w: duplicate slot %d", errShowcaseInvalidDraft, item.SlotNo)
		}
		slots[item.SlotNo] = struct{}{}
		if _, exists := products[item.ProductID]; exists {
			return fmt.Errorf("%w: duplicate product %d", errShowcaseInvalidDraft, item.ProductID)
		}
		products[item.ProductID] = struct{}{}
	}
	return nil
}

func buildAdminShowcase(layout ShowcaseResp, cards map[int64]ProductCard) ShowcaseResp {
	for index := range layout.Items {
		card, exists := cards[layout.Items[index].ProductID]
		if !exists {
			continue
		}
		cardCopy := card
		cardCopy.SlotNo = layout.Items[index].SlotNo
		layout.Items[index].Product = &cardCopy
	}
	return layout
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

func publishShowcase(ctx context.Context, db *sql.DB, operatorID int64, req showcasePublishReq) (int64, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	var currentVersion int64
	if err := tx.QueryRowContext(ctx, `SELECT version FROM mall_product.homepage_showcase WHERE id = ? FOR UPDATE`, currentShowcaseID).Scan(&currentVersion); err != nil {
		return 0, err
	}
	if currentVersion != req.ExpectedVersion {
		return 0, fmt.Errorf("%w: current_version=%d", errShowcaseVersionConflict, currentVersion)
	}

	states := make(map[int64]showcaseProductState, len(req.Items))
	if len(req.Items) > 0 {
		placeholders := make([]string, 0, len(req.Items))
		args := make([]any, 0, len(req.Items))
		for _, item := range req.Items {
			placeholders = append(placeholders, "?")
			args = append(args, item.ProductID)
		}
		rows, err := tx.QueryContext(ctx, fmt.Sprintf(`SELECT product.id, product.status, product.merchant_id,
       IF(merchant.id IS NULL, 0, 1), COALESCE(merchant.status, 0),
       COALESCE(stock.available, product.stock, 0)
FROM mall_product.product product
LEFT JOIN mall_order.merchant merchant ON merchant.id = product.merchant_id
LEFT JOIN mall_product.product_stock_snapshot stock ON stock.product_id = product.id
WHERE product.id IN (%s)`, strings.Join(placeholders, ",")), args...)
		if err != nil {
			return 0, err
		}
		for rows.Next() {
			var state showcaseProductState
			var merchantExists int64
			if err := rows.Scan(&state.ProductID, &state.ProductStatus, &state.MerchantID,
				&merchantExists, &state.MerchantStatus, &state.StockAvailable); err != nil {
				_ = rows.Close()
				return 0, err
			}
			state.ProductExists = true
			state.MerchantExists = merchantExists == 1
			states[state.ProductID] = state
		}
		if err := rows.Close(); err != nil {
			return 0, err
		}
		if err := rows.Err(); err != nil {
			return 0, err
		}
	}
	if err := validateShowcaseDraft(req.Items, states); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM mall_product.homepage_showcase_item WHERE showcase_id = ?`, currentShowcaseID); err != nil {
		return 0, err
	}
	if len(req.Items) > 0 {
		values := make([]string, 0, len(req.Items))
		args := make([]any, 0, len(req.Items)*3)
		for _, item := range req.Items {
			values = append(values, "(?, ?, ?)")
			args = append(args, currentShowcaseID, item.SlotNo, item.ProductID)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO mall_product.homepage_showcase_item (showcase_id, slot_no, product_id) VALUES `+strings.Join(values, ","), args...); err != nil {
			return 0, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mall_product.homepage_showcase
SET version = version + 1, operator_id = ?, publish_time = NOW()
WHERE id = ?`, operatorID, currentShowcaseID); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return currentVersion + 1, nil
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

func loadAdminShowcase(ctx context.Context, svcCtx *svc.ServiceContext, db *sql.DB) (ShowcaseResp, error) {
	layout, err := loadShowcaseLayout(ctx, db)
	if err != nil {
		return ShowcaseResp{}, err
	}
	productIDs := make([]int64, 0, 12)
	for _, slot := range layout.Items {
		if !slot.Empty && slot.ProductID > 0 && slot.InvalidReason != "product_not_found" {
			productIDs = append(productIDs, slot.ProductID)
		}
	}
	if len(productIDs) == 0 {
		return layout, nil
	}
	resp, err := svcCtx.ProductRpc.ListProducts(ctx, &productclient.ListProductsReq{ProductIds: productIDs})
	if err != nil {
		return ShowcaseResp{}, err
	}
	cards := buildProductCards(resp.Items, loadProductMeta(ctx, svcCtx, productIDs), nil)
	return buildAdminShowcase(layout, cards), nil
}

func AdminShowcaseHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
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
		layout, err := loadAdminShowcase(ctx, svcCtx, db)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "showcase query failed", err))
			return
		}
		ok(ctx, c, layout)
	}
}

func AdminShowcasePublishHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req showcasePublishReq
		if err := decodeJSONBody(c, &req); err != nil || req.ExpectedVersion <= 0 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid showcase publish request"))
			return
		}
		if err := validateShowcaseDraftShape(req.Items); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, err.Error()))
			return
		}
		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "product datasource unavailable", err))
			return
		}
		if err = ensureStorefrontSchema(ctx, db); err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "showcase schema unavailable", err))
			return
		}
		operatorID := gatewayOperatorID(ctx)
		newVersion, err := publishShowcase(ctx, db, operatorID, req)
		if errors.Is(err, errShowcaseInvalidDraft) {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, err.Error()))
			return
		}
		if errors.Is(err, errShowcaseVersionConflict) || errors.Is(err, errShowcaseStateConflict) {
			fail(ctx, c, consts.StatusConflict, apperror.New(apperror.CodeConflict, err.Error()))
			return
		}
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "showcase publish failed", err))
			return
		}
		productIDs := make([]string, 0, len(req.Items))
		for _, item := range req.Items {
			productIDs = append(productIDs, fmt.Sprintf("%d", item.ProductID))
		}
		recordGatewayAdminAuditEvent(c, svcCtx, adminAuditHomepageShowcasePublished,
			fmt.Sprintf("old_version:%d new_version:%d operator:%d products:%s", req.ExpectedVersion, newVersion, operatorID, strings.Join(productIDs, ",")))
		layout, err := loadAdminShowcase(ctx, svcCtx, db)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "showcase query failed", err))
			return
		}
		ok(ctx, c, layout)
	}
}
