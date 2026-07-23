package handler

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/application/promotion"
	"flash-mall/app/gateway/hertz/internal/ports"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

var errPromotionServiceUnavailable = errors.New("promotion service unavailable")

func AdminPromotionListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		query, appErr := parsePromotionListQuery(c)
		if appErr != nil {
			fail(ctx, c, consts.StatusBadRequest, appErr)
			return
		}
		if svcCtx.Promotions == nil {
			failPromotion(ctx, c, "admin promotion query failed", errPromotionServiceUnavailable)
			return
		}
		items, total, err := svcCtx.Promotions.List(ctx, query)
		if err != nil {
			failPromotion(ctx, c, "admin promotion query failed", err)
			return
		}
		ok(ctx, c, AdminPromotionListResp{Items: items, Total: total, Page: query.Page, PageSize: query.PageSize})
	}
}

func AdminPromotionDetailHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		promotionID, err := parsePositiveInt64(c.Query("promotion_id"))
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "promotion_id required"))
			return
		}
		if svcCtx.Promotions == nil {
			failPromotion(ctx, c, "admin promotion detail query failed", errPromotionServiceUnavailable)
			return
		}
		item, err := svcCtx.Promotions.Detail(ctx, promotionID)
		if err != nil {
			failPromotion(ctx, c, "admin promotion detail query failed", err)
			return
		}
		ok(ctx, c, item)
	}
}

func AdminPromotionCreateHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var request AdminPromotionCreateReq
		if err := decodeJSONBody(c, &request); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid promotion create request"))
			return
		}
		if svcCtx.Promotions == nil {
			failPromotion(ctx, c, "admin promotion create failed", errPromotionServiceUnavailable)
			return
		}
		result, err := svcCtx.Promotions.Create(ctx, request)
		if err != nil {
			recordPromotionMutationFailure(c, svcCtx, adminAuditPromotionCreated, 0, request.ProductID, err)
			failPromotion(ctx, c, "admin promotion create failed", err)
			return
		}
		refreshProductCardSnapshotsBestEffort(ctx, svcCtx, result.AffectedProductIDs...)
		recordGatewayAdminAuditEvent(c, svcCtx, adminAuditPromotionCreated, fmt.Sprintf("promotion:%d product:%d", result.PromotionID, request.ProductID))
		ok(ctx, c, AdminPromotionCreateResp{PromotionID: result.PromotionID})
	}
}

func AdminPromotionUpdateHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var request AdminPromotionUpdateReq
		if err := decodeJSONBody(c, &request); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid promotion update request"))
			return
		}
		if svcCtx.Promotions == nil {
			failPromotion(ctx, c, "admin promotion update failed", errPromotionServiceUnavailable)
			return
		}
		result, err := svcCtx.Promotions.Update(ctx, request)
		if err != nil {
			productID := int64(0)
			if request.ProductID != nil {
				productID = *request.ProductID
			}
			recordPromotionMutationFailure(c, svcCtx, promotionUpdateAuditEvent(request.Status), request.PromotionID, productID, err)
			failPromotion(ctx, c, "admin promotion update failed", err)
			return
		}
		refreshProductCardSnapshotsBestEffort(ctx, svcCtx, result.AffectedProductIDs...)
		recordGatewayAdminAuditEvent(c, svcCtx, promotionUpdateAuditEvent(request.Status), fmt.Sprintf("promotion:%d", request.PromotionID))
		ok(ctx, c, map[string]any{"ok": true})
	}
}

func parsePromotionListQuery(c *app.RequestContext) (promotion.ListQuery, *apperror.Error) {
	page, err := parseInt64Default(c.Query("page"), defaultProductPage)
	if err != nil || page <= 0 {
		return promotion.ListQuery{}, apperror.New(apperror.CodeInvalidArgument, "page must be positive")
	}
	pageSize, err := parseInt64Default(c.Query("page_size"), defaultProductPageSize)
	if err != nil || pageSize <= 0 {
		return promotion.ListQuery{}, apperror.New(apperror.CodeInvalidArgument, "page_size must be positive")
	}
	if pageSize > maxProductPageSize {
		pageSize = maxProductPageSize
	}
	productID, err := parseOptionalInt64(c.Query("product_id"))
	if err != nil {
		return promotion.ListQuery{}, apperror.New(apperror.CodeInvalidArgument, "product_id must be numeric")
	}
	status, err := parseInt64Default(c.Query("status"), -1)
	if err != nil {
		return promotion.ListQuery{}, apperror.New(apperror.CodeInvalidArgument, "status must be numeric")
	}
	return promotion.ListQuery{
		Page: page, PageSize: pageSize, ProductID: productID, Status: status,
		EffectStatus: strings.TrimSpace(c.Query("effect_status")), Keyword: strings.TrimSpace(c.Query("keyword")),
	}, nil
}

func failPromotion(ctx context.Context, c *app.RequestContext, operation string, err error) {
	fault, ok := promotion.AsFault(err)
	if !ok {
		fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, operation, err))
		return
	}
	switch fault.Reason {
	case promotion.ReasonProductNotFound:
		fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeProductNotFound, fault.Message))
	case promotion.ReasonPromotionNotFound:
		fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeNotFound, fault.Message))
	case promotion.ReasonWindowConflict:
		fail(ctx, c, consts.StatusConflict, apperror.New(apperror.CodeConflict, fault.Message))
	default:
		fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, fault.Message))
	}
}

func recordPromotionMutationFailure(c *app.RequestContext, svcCtx *svc.ServiceContext, event string, promotionID, productID int64, err error) {
	fault, ok := promotion.AsFault(err)
	if !ok || fault.Reason == promotion.ReasonInvalidArgument {
		return
	}
	detail := ""
	if promotionID > 0 {
		detail = fmt.Sprintf("promotion:%d", promotionID)
	}
	if productID > 0 {
		if detail != "" {
			detail += " "
		}
		detail += fmt.Sprintf("product:%d", productID)
	}
	if detail != "" {
		detail += " "
	}
	detail += "reason:" + fault.Reason
	recordGatewayAdminAuditFailure(c, svcCtx, event, detail)
}

func refreshProductCardSnapshotsBestEffort(ctx context.Context, svcCtx *svc.ServiceContext, productIDs ...int64) {
	store, err := requireProductSnapshotStore(svcCtx)
	if err != nil {
		return
	}
	for _, productID := range uniquePositiveInt64s(productIDs) {
		_, _ = store.RebuildCards(ctx, ports.SnapshotRebuildRequest{ProductID: productID, Limit: 1})
	}
}
