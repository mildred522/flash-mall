package handler

import (
	"context"
	"fmt"
	"strings"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/common/orderstatus"
	"flash-mall/app/common/tracectx"
	"flash-mall/app/gateway/hertz/internal/ports"
	"flash-mall/app/gateway/hertz/internal/svc"
	orderpb "flash-mall/app/order/rpc/order"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func AdminOrderListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		req, err := adminOrderQueryFromRequest(c)
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, err)
			return
		}
		service, err := backofficeOrderService(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order datasource unavailable", err))
			return
		}
		resp, err := service.ListAdminOrders(ctx, req)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin order query failed", err))
			return
		}
		ok(ctx, c, resp)
	}
}

func AdminOrderDetailHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		orderID := strings.TrimSpace(c.Query("order_id"))
		if orderID == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "order_id is required"))
			return
		}
		service, err := backofficeOrderService(svcCtx)
		if err == nil {
			resp, queryErr := service.AdminDetail(ctx, orderID)
			if queryErr != nil {
				fail(ctx, c, createOrderStatusCode(queryErr), queryErr)
				return
			}
			ok(ctx, c, resp)
			return
		}
		if err != nil {
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
	}
}

func AdminOrderStatusLogHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		orderID := strings.TrimSpace(c.Query("order_id"))
		if orderID == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "order_id is required"))
			return
		}
		service, err := backofficeOrderService(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order datasource unavailable", err))
			return
		}
		resp, err := service.AdminStatusLogs(ctx, orderID)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin order status log query failed", err))
			return
		}
		ok(ctx, c, resp)
	}
}

func AdminShipOrderHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req MerchantShipOrderReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid ship order request"))
			return
		}
		req.OrderID = strings.TrimSpace(req.OrderID)
		if req.OrderID == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "order_id is required"))
			return
		}
		operatorID := gatewayOperatorID(ctx)
		commands, err := requireOrderCommands(svcCtx)
		if err == nil {
			err = commands.ShipAdmin(ctx, ports.ShipAdminOrderCommand{
				OrderID: req.OrderID, OperatorID: operatorID, Meta: inventoryRequestMeta(ctx),
			})
		}
		if err != nil {
			recordGatewayAdminAuditFailure(c, svcCtx, adminAuditOrderShipped, fmt.Sprintf("order:%s reason:%s", req.OrderID, adminAuditReasonInvalidStatus))
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		recordGatewayAdminAuditEvent(c, svcCtx, adminAuditOrderShipped, fmt.Sprintf("order:%s operator:%d", req.OrderID, operatorID))
		ok(ctx, c, MerchantShipOrderResp{OrderID: req.OrderID, Status: orderstatus.Text(orderstatus.Shipped)})
	}
}

func AdminCloseOrderHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req CancelOrderReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid close order request"))
			return
		}
		req.OrderID = strings.TrimSpace(req.OrderID)
		req.Reason = strings.TrimSpace(req.Reason)
		if req.OrderID == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "order_id is required"))
			return
		}
		if req.Reason == "" {
			req.Reason = "admin close"
		}
		operatorID := gatewayOperatorID(ctx)
		commands, err := requireOrderCommands(svcCtx)
		if err == nil {
			err = commands.CloseAdmin(ctx, ports.CloseAdminOrderCommand{
				OrderID: req.OrderID, Reason: req.Reason, OperatorID: operatorID, Meta: inventoryRequestMeta(ctx),
			})
		}
		if err != nil {
			recordGatewayAdminAuditFailure(c, svcCtx, adminAuditOrderClosed, fmt.Sprintf("order:%s reason:%s", req.OrderID, adminAuditReasonInvalidStatus))
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		recordGatewayAdminAuditEvent(c, svcCtx, adminAuditOrderClosed, fmt.Sprintf("order:%s operator:%d reason:%s", req.OrderID, operatorID, req.Reason))
		ok(ctx, c, CancelOrderResp{OrderID: req.OrderID, Status: orderstatus.Text(orderstatus.Closed)})
	}
}

func AdminRefundOrderHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req RefundOrderReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid admin refund request"))
			return
		}
		req.OrderID = strings.TrimSpace(req.OrderID)
		req.Reason = strings.TrimSpace(req.Reason)
		if req.OrderID == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "order_id is required"))
			return
		}
		if req.Reason == "" {
			req.Reason = "admin refund"
		}
		operatorID := gatewayOperatorID(ctx)
		if err := refundAdminOrder(ctx, svcCtx, req, operatorID); err != nil {
			recordGatewayAdminAuditFailure(c, svcCtx, adminAuditOrderRefunded, fmt.Sprintf("order:%s reason:%s", req.OrderID, adminAuditReasonInvalidStatus))
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		recordGatewayAdminAuditEvent(c, svcCtx, adminAuditOrderRefunded, fmt.Sprintf("order:%s operator:%d reason:%s", req.OrderID, operatorID, req.Reason))
		ok(ctx, c, RefundOrderResp{OrderID: req.OrderID, Status: orderstatus.Text(orderstatus.Refunded)})
	}
}

func adminOrderQueryFromRequest(c *app.RequestContext) (AdminOrderListReq, error) {
	page, err := parseInt64Default(c.Query("page"), 1)
	if err != nil {
		return AdminOrderListReq{}, apperror.New(apperror.CodeInvalidArgument, "page must be numeric")
	}
	pageSize, err := parseInt64Default(c.Query("page_size"), 20)
	if err != nil {
		return AdminOrderListReq{}, apperror.New(apperror.CodeInvalidArgument, "page_size must be numeric")
	}
	status, err := parseInt64Default(c.Query("status"), -1)
	if err != nil {
		return AdminOrderListReq{}, apperror.New(apperror.CodeInvalidArgument, "status must be numeric")
	}
	merchantID, err := parseOptionalInt64(c.Query("merchant_id"))
	if err != nil {
		return AdminOrderListReq{}, apperror.New(apperror.CodeInvalidArgument, "merchant_id must be numeric")
	}
	userID, err := parseOptionalInt64(c.Query("user_id"))
	if err != nil {
		return AdminOrderListReq{}, apperror.New(apperror.CodeInvalidArgument, "user_id must be numeric")
	}
	productID, err := parseOptionalInt64(c.Query("product_id"))
	if err != nil {
		return AdminOrderListReq{}, apperror.New(apperror.CodeInvalidArgument, "product_id must be numeric")
	}
	return AdminOrderListReq{
		Page:        normalizePage(page),
		PageSize:    normalizeAdminPageSize(pageSize),
		MerchantID:  merchantID,
		UserID:      userID,
		ProductID:   productID,
		Status:      status,
		ProductName: strings.TrimSpace(c.Query("product_name")),
		CreatedFrom: strings.TrimSpace(c.Query("created_from")),
		CreatedTo:   strings.TrimSpace(c.Query("created_to")),
		OrderID:     strings.TrimSpace(c.Query("order_id")),
	}, nil
}

func refundAdminOrder(ctx context.Context, svcCtx *svc.ServiceContext, req RefundOrderReq, operatorID int64) error {
	requestID := tracectx.RequestIDFrom(ctx)
	if requestID == "" {
		requestID = req.OrderID + ":admin-refund"
	}
	requested, err := svcCtx.OrderRpc.RequestRefund(ctx, &orderpb.RequestRefundReq{
		OrderId: req.OrderID, RequesterId: operatorID, RequesterRole: "admin",
		Reason: req.Reason, RequestId: requestID,
	})
	if err != nil {
		return err
	}
	_, err = svcCtx.OrderRpc.AuditRefund(ctx, &orderpb.AuditRefundReq{
		RefundId: requested.GetRefundId(), OperatorId: operatorID, Approve: true,
		Remark: req.Reason, RequestId: requestID,
	})
	return err
}

func gatewayOperatorID(ctx context.Context) int64 {
	if identity, ok := authctx.IdentityFrom(ctx); ok && identity.UserID > 0 {
		return identity.UserID
	}
	return 0
}

func normalizeAdminPageSize(pageSize int64) int64 {
	if pageSize <= 0 || pageSize > 100 {
		return 20
	}
	return pageSize
}
