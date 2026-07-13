package handler

import (
	"context"
	"database/sql"
	"strings"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/tracectx"
	"flash-mall/app/gateway/hertz/internal/svc"
	orderpb "flash-mall/app/order/rpc/order"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func AdminRefundListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		req, err := adminRefundQueryFromRequest(c)
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, err)
			return
		}
		db, err := orderDB(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order datasource unavailable", err))
			return
		}
		resp, err := loadAdminRefunds(ctx, db, req)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin refund query failed", err))
			return
		}
		ok(ctx, c, resp)
	}
}

func AdminRefundAuditHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req AdminRefundAuditReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid refund audit request"))
			return
		}
		req.RefundID = strings.TrimSpace(req.RefundID)
		req.Remark = strings.TrimSpace(req.Remark)
		if req.RefundID == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "refund_id is required"))
			return
		}
		operatorID := gatewayOperatorID(ctx)
		statusText, err := auditAdminRefund(ctx, svcCtx, nil, req, operatorID)
		if err != nil {
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		ok(ctx, c, AdminRefundAuditResp{RefundID: req.RefundID, Status: statusText})
	}
}

func adminRefundQueryFromRequest(c *app.RequestContext) (AdminRefundListReq, error) {
	page, err := parseInt64Default(c.Query("page"), 1)
	if err != nil {
		return AdminRefundListReq{}, apperror.New(apperror.CodeInvalidArgument, "page must be numeric")
	}
	pageSize, err := parseInt64Default(c.Query("page_size"), 20)
	if err != nil {
		return AdminRefundListReq{}, apperror.New(apperror.CodeInvalidArgument, "page_size must be numeric")
	}
	status, err := parseInt64Default(c.Query("status"), -1)
	if err != nil {
		return AdminRefundListReq{}, apperror.New(apperror.CodeInvalidArgument, "status must be numeric")
	}
	merchantID, err := parseOptionalInt64(c.Query("merchant_id"))
	if err != nil {
		return AdminRefundListReq{}, apperror.New(apperror.CodeInvalidArgument, "merchant_id must be numeric")
	}
	userID, err := parseOptionalInt64(c.Query("user_id"))
	if err != nil {
		return AdminRefundListReq{}, apperror.New(apperror.CodeInvalidArgument, "user_id must be numeric")
	}
	return AdminRefundListReq{
		Page:       normalizePage(page),
		PageSize:   normalizeAdminPageSize(pageSize),
		MerchantID: merchantID,
		Status:     status,
		UserID:     userID,
		OrderID:    strings.TrimSpace(c.Query("order_id")),
	}, nil
}

func loadAdminRefunds(ctx context.Context, db *sql.DB, req AdminRefundListReq) (AdminRefundListResp, error) {
	where := "1=1"
	args := []any{}
	if req.Status >= 0 {
		where += " AND r.status = ?"
		args = append(args, req.Status)
	}
	if req.UserID > 0 {
		where += " AND r.user_id = ?"
		args = append(args, req.UserID)
	}
	if req.MerchantID > 0 {
		where += " AND r.merchant_id = ?"
		args = append(args, req.MerchantID)
	}
	if req.OrderID != "" {
		where += " AND r.order_id = ?"
		args = append(args, req.OrderID)
	}
	var total int64
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM refund_order r WHERE "+where, args...).Scan(&total); err != nil {
		return AdminRefundListResp{}, err
	}
	queryArgs := append(append([]any{}, args...), req.PageSize, (req.Page-1)*req.PageSize)
	rows, err := db.QueryContext(ctx, `SELECT r.id,
       r.order_id,
       r.payment_order_id,
       r.user_id,
       r.merchant_id,
       COALESCE(m.name, ''),
       r.product_id,
       r.refund_amount_fen,
       r.status,
       r.reason,
       r.audit_remark,
       r.operator_id,
       DATE_FORMAT(r.request_time, '%Y-%m-%d %H:%i:%s'),
       COALESCE(DATE_FORMAT(r.audit_time, '%Y-%m-%d %H:%i:%s'), ''),
       COALESCE(DATE_FORMAT(r.finish_time, '%Y-%m-%d %H:%i:%s'), '')
FROM refund_order r
LEFT JOIN merchant m ON m.id = r.merchant_id
WHERE `+where+`
ORDER BY r.create_time DESC
LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return AdminRefundListResp{}, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]AdminRefundItem, 0)
	for rows.Next() {
		var item AdminRefundItem
		if err := rows.Scan(&item.RefundID, &item.OrderID, &item.PaymentOrderID, &item.UserID, &item.MerchantID, &item.MerchantName, &item.ProductID, &item.RefundAmountFen, &item.Status, &item.Reason, &item.AuditRemark, &item.OperatorID, &item.RequestTime, &item.AuditTime, &item.FinishTime); err != nil {
			return AdminRefundListResp{}, err
		}
		item.StatusText = refundStatusText(item.Status)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return AdminRefundListResp{}, err
	}
	return AdminRefundListResp{Items: items, Total: total}, nil
}

func auditAdminRefund(ctx context.Context, svcCtx *svc.ServiceContext, _ *sql.DB, req AdminRefundAuditReq, operatorID int64) (string, error) {
	requestID := tracectx.RequestIDFrom(ctx)
	if requestID == "" {
		requestID = req.RefundID + ":audit"
	}
	resp, err := svcCtx.OrderRpc.AuditRefund(ctx, &orderpb.AuditRefundReq{
		RefundId: req.RefundID, OperatorId: operatorID, Approve: req.Approve,
		Remark: req.Remark, RequestId: requestID,
	})
	if err != nil {
		return "", err
	}
	return refundStatusText(resp.GetRefundStatus()), nil
}
