package handler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/tracectx"
	"flash-mall/app/gateway/hertz/internal/application/showcase"
	"flash-mall/app/gateway/hertz/internal/svc"
	"flash-mall/app/product/rpc/productclient"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/zeromicro/go-zero/core/logx"
)

var (
	errShowcaseInvalidDraft    = showcase.ErrInvalidDraft
	errShowcaseStateConflict   = showcase.ErrStateConflict
	errShowcaseVersionConflict = showcase.ErrVersionConflict
	errShowcaseServiceMissing  = errors.New("showcase service unavailable")
)

type showcasePublishItem = showcase.PublishItem
type showcasePublishReq = showcase.PublishInput
type showcaseProductState = showcase.ProductState
type showcaseSlotState = showcase.SlotState

func deriveShowcaseInvalidReason(state showcaseSlotState) string {
	return showcase.InvalidReason(state)
}

func validateShowcaseDraft(items []showcasePublishItem, states map[int64]showcaseProductState) error {
	return showcase.ValidateDraft(items, states)
}

func validateShowcaseDraftShape(items []showcasePublishItem) error {
	return showcase.ValidateDraftShape(items)
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
		startedAt := time.Now()
		result := "error"
		var observedLayout ShowcaseResp
		defer func() { recordShowcaseRead("public", result, observedLayout, time.Since(startedAt)) }()
		catalog, source, err := loadCachedJSON(ctx, svcCtx, showcaseCatalogCacheKey, func(loadCtx context.Context) (ProductListResp, error) {
			layout, loadErr := loadShowcaseLayoutFromService(loadCtx, svcCtx)
			if loadErr != nil {
				return ProductListResp{}, apperror.Wrap(apperror.CodeInternal, "showcase query failed", loadErr)
			}
			observedLayout = layout
			productIDs := validShowcaseProductIDs(layout)
			if len(productIDs) == 0 {
				return buildPublicShowcaseCatalog(layout, map[int64]ProductCard{}), nil
			}
			resp, rpcErr := svcCtx.ProductRpc.ListProducts(loadCtx, &productclient.ListProductsReq{ProductIds: productIDs})
			if rpcErr != nil {
				return ProductListResp{}, apperror.Wrap(apperror.CodeInternal, "product service unavailable", rpcErr)
			}
			cards := buildProductCards(resp.Items, loadProductMeta(loadCtx, svcCtx, productIDs), nil)
			return buildPublicShowcaseCatalog(layout, cards), nil
		})
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		if source != "origin" {
			observedLayout = showcaseLayoutFromCatalog(catalog)
		}
		result = "success"
		ok(ctx, c, catalog)
	}
}

func showcaseLayoutFromCatalog(catalog ProductListResp) ShowcaseResp {
	layout := emptyHandlerShowcase()
	for _, card := range catalog.Items {
		if card.SlotNo >= 1 && card.SlotNo <= 12 {
			layout.Items[card.SlotNo-1] = ShowcaseSlot{SlotNo: card.SlotNo, ProductID: card.ProductID, Valid: true}
		}
	}
	return layout
}

func loadAdminShowcase(ctx context.Context, svcCtx *svc.ServiceContext) (ShowcaseResp, error) {
	layout, err := loadShowcaseLayoutFromService(ctx, svcCtx)
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
		startedAt := time.Now()
		result := "error"
		var observedLayout ShowcaseResp
		defer func() { recordShowcaseRead("admin", result, observedLayout, time.Since(startedAt)) }()
		layout, err := loadAdminShowcase(ctx, svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "showcase query failed", err))
			return
		}
		observedLayout = layout
		result = "success"
		ok(ctx, c, layout)
	}
}

func AdminShowcasePublishHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		startedAt := time.Now()
		publishResult := "error"
		operatorID := gatewayOperatorID(ctx)
		var expectedVersion, currentVersion, newVersion int64
		defer func() {
			recordShowcasePublish(publishResult)
			logx.WithContext(ctx).Infof(
				"showcase_publish result=%s operator_id=%d expected_version=%d current_version=%d new_version=%d request_id=%s duration_ms=%d",
				publishResult, operatorID, expectedVersion, currentVersion, newVersion,
				tracectx.RequestIDFrom(ctx), time.Since(startedAt).Milliseconds(),
			)
		}()
		var request showcasePublishReq
		if err := decodeJSONBody(c, &request); err != nil || request.ExpectedVersion <= 0 {
			publishResult = "invalid"
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid showcase publish request"))
			return
		}
		expectedVersion, currentVersion = request.ExpectedVersion, request.ExpectedVersion
		if err := showcase.ValidateDraftShape(request.Items); err != nil {
			publishResult = "invalid"
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, err.Error()))
			return
		}
		if svcCtx.Showcases == nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "showcase publish failed", errShowcaseServiceMissing))
			return
		}
		var err error
		newVersion, err = svcCtx.Showcases.Publish(ctx, operatorID, request)
		if errors.Is(err, errShowcaseInvalidDraft) {
			publishResult = "invalid"
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, err.Error()))
			return
		}
		if errors.Is(err, errShowcaseVersionConflict) || errors.Is(err, errShowcaseStateConflict) {
			publishResult = "conflict"
			if latest, loadErr := svcCtx.Showcases.Load(ctx); loadErr == nil {
				currentVersion = latest.Version
			}
			fail(ctx, c, consts.StatusConflict, apperror.New(apperror.CodeConflict, err.Error()))
			return
		}
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "showcase publish failed", err))
			return
		}
		publishResult = "success"
		invalidateShowcaseCaches(ctx, svcCtx)
		productIDs := make([]string, 0, len(request.Items))
		for _, item := range request.Items {
			productIDs = append(productIDs, fmt.Sprintf("%d", item.ProductID))
		}
		recordGatewayAdminAuditEvent(c, svcCtx, adminAuditHomepageShowcasePublished,
			fmt.Sprintf("old_version:%d new_version:%d operator:%d products:%s", request.ExpectedVersion, newVersion, operatorID, strings.Join(productIDs, ",")))
		layout, err := loadAdminShowcase(ctx, svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "showcase query failed", err))
			return
		}
		ok(ctx, c, layout)
	}
}

func loadShowcaseLayoutFromService(ctx context.Context, svcCtx *svc.ServiceContext) (ShowcaseResp, error) {
	if svcCtx.Showcases == nil {
		return ShowcaseResp{}, errShowcaseServiceMissing
	}
	layout, err := svcCtx.Showcases.Load(ctx)
	if err != nil {
		return ShowcaseResp{}, err
	}
	return handlerShowcaseLayout(layout), nil
}

func handlerShowcaseLayout(layout showcase.Layout) ShowcaseResp {
	result := ShowcaseResp{Version: layout.Version, OperatorID: layout.OperatorID, PublishTime: layout.PublishTime, Items: make([]ShowcaseSlot, len(layout.Items))}
	for index, slot := range layout.Items {
		result.Items[index] = ShowcaseSlot{SlotNo: slot.SlotNo, ProductID: slot.ProductID, Empty: slot.Empty, Valid: slot.Valid, InvalidReason: slot.InvalidReason}
	}
	return result
}

func emptyHandlerShowcase() ShowcaseResp {
	layout := ShowcaseResp{Items: make([]ShowcaseSlot, 12)}
	for index := range layout.Items {
		layout.Items[index] = ShowcaseSlot{SlotNo: int64(index + 1), Empty: true}
	}
	return layout
}

func validShowcaseProductIDs(layout ShowcaseResp) []int64 {
	productIDs := make([]int64, 0, 12)
	for _, slot := range layout.Items {
		if slot.Valid {
			productIDs = append(productIDs, slot.ProductID)
		}
	}
	return productIDs
}

func invalidateShowcaseCaches(ctx context.Context, svcCtx *svc.ServiceContext) {
	if svcCtx.Cache == nil {
		return
	}
	if err := svcCtx.Cache.Invalidate(ctx, showcaseCatalogCacheKey); err != nil {
		logx.WithContext(ctx).Errorf("gateway showcase cache invalidation failed: %v", err)
	}
	if err := svcCtx.Cache.InvalidatePrefix(ctx, "showcase:candidates:"); err != nil {
		logx.WithContext(ctx).Errorf("gateway showcase candidate cache invalidation failed: %v", err)
	}
}
