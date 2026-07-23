package handler

import (
	"context"
	"strings"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/refundstatus"
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
		if svcCtx.BackofficeOrders == nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.New(apperror.CodeInternal, "order query service unavailable"))
			return
		}
		resp, err := svcCtx.BackofficeOrders.ListAdminRefunds(ctx, req)
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
		statusText, err := auditAdminRefund(ctx, svcCtx, req, operatorID)
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

func auditAdminRefund(ctx context.Context, svcCtx *svc.ServiceContext, req AdminRefundAuditReq, operatorID int64) (string, error) {
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
	return refundstatus.Text(resp.GetRefundStatus()), nil
}
