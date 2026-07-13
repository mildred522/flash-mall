package handler

import (
	"context"
	"strings"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func AdminStockChangeLogHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		writeStockChangeLog(ctx, c, svcCtx, 0)
	}
}

func MerchantStockChangeLogHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, found := authctx.IdentityFrom(ctx)
		if !found || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}
		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		merchantID, err := selectedMerchantID(ctx, db, identity)
		if err != nil {
			fail(ctx, c, consts.StatusForbidden, err)
			return
		}
		writeStockChangeLog(ctx, c, svcCtx, merchantID)
	}
}

func writeStockChangeLog(ctx context.Context, c *app.RequestContext, svcCtx *svc.ServiceContext, merchantID int64) {
	db, err := svcCtx.SqlConn.RawDB()
	if err != nil {
		fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "stock audit datasource unavailable", err))
		return
	}
	page, err := parseInt64Default(c.Query("page"), 1)
	if err != nil {
		fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "page must be numeric"))
		return
	}
	pageSize, err := parseInt64Default(c.Query("page_size"), 20)
	if err != nil {
		fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "page_size must be numeric"))
		return
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	where, args := "1=1", []any{}
	if productID, err := parseInt64Default(c.Query("product_id"), 0); err != nil {
		fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "product_id must be numeric"))
		return
	} else if productID > 0 {
		where += " AND l.product_id = ?"
		args = append(args, productID)
	}
	if orderID := strings.TrimSpace(c.Query("order_id")); orderID != "" {
		where += " AND l.order_id = ?"
		args = append(args, orderID)
	}
	if changeType := strings.ToUpper(strings.TrimSpace(c.Query("change_type"))); changeType != "" {
		where += " AND l.change_type = ?"
		args = append(args, changeType)
	}
	if merchantID > 0 {
		where += " AND EXISTS (SELECT 1 FROM mall_product.product p WHERE p.id = l.product_id AND p.merchant_id = ?)"
		args = append(args, merchantID)
	}
	var total int64
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mall_product.inventory_stock_change_log l WHERE "+where, args...).Scan(&total); err != nil {
		fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "stock audit count failed", err))
		return
	}
	queryArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := db.QueryContext(ctx, "SELECT id, product_id, order_id, change_type, delta, before_available, after_available, reason, request_id, trace_id, operator_user_id, operator_merchant_id, operator_role, COALESCE(DATE_FORMAT(create_time, '%Y-%m-%d %H:%i:%s'), '') FROM mall_product.inventory_stock_change_log l WHERE "+where+" ORDER BY id DESC LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "stock audit query failed", err))
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var id, productID, delta, beforeAvailable, afterAvailable, operatorUserID, operatorMerchantID int64
		var orderID, changeType, reason, requestID, traceID, operatorRole, createTime string
		if err := rows.Scan(&id, &productID, &orderID, &changeType, &delta, &beforeAvailable, &afterAvailable, &reason, &requestID, &traceID, &operatorUserID, &operatorMerchantID, &operatorRole, &createTime); err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		items = append(items, map[string]any{"id": id, "product_id": productID, "order_id": orderID, "change_type": changeType, "delta": delta, "before_available": beforeAvailable, "after_available": afterAvailable, "reason": reason, "request_id": requestID, "trace_id": traceID, "operator_user_id": operatorUserID, "operator_merchant_id": operatorMerchantID, "operator_role": operatorRole, "create_time": createTime})
	}
	if err := rows.Err(); err != nil {
		fail(ctx, c, consts.StatusBadGateway, err)
		return
	}
	ok(ctx, c, map[string]any{"items": items, "total": total, "page": page, "page_size": pageSize})
}
