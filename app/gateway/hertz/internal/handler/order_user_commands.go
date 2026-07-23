package handler

import (
	"context"
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

func CancelOrderHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "user login required"))
			return
		}

		var req CancelOrderReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid order cancel request"))
			return
		}
		req.OrderID = strings.TrimSpace(req.OrderID)
		req.Reason = strings.TrimSpace(req.Reason)
		if req.OrderID == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "order_id is required"))
			return
		}
		if req.Reason == "" {
			req.Reason = "user cancel"
		}

		commands, err := requireOrderCommands(svcCtx)
		if err == nil {
			err = commands.CancelUser(ctx, ports.CancelUserOrderCommand{
				OrderID: req.OrderID, Reason: req.Reason, UserID: identity.UserID, Meta: inventoryRequestMeta(ctx),
			})
		}
		if err != nil {
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		ok(ctx, c, CancelOrderResp{OrderID: req.OrderID, Status: orderstatus.Text(orderstatus.Closed)})
	}
}

func RefundOrderHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "user login required"))
			return
		}

		var req RefundOrderReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid order refund request"))
			return
		}
		req.OrderID = strings.TrimSpace(req.OrderID)
		req.Reason = strings.TrimSpace(req.Reason)
		if req.OrderID == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "order_id is required"))
			return
		}
		if req.Reason == "" {
			req.Reason = "user refund"
		}

		if err := requestUserRefund(ctx, svcCtx, req, identity.UserID); err != nil {
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		ok(ctx, c, RefundOrderResp{OrderID: req.OrderID, Status: orderstatus.Text(orderstatus.RefundRequested)})
	}
}

func ConfirmReceiptHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "user login required"))
			return
		}

		var req ConfirmReceiptReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid confirm receipt request"))
			return
		}
		req.OrderID = strings.TrimSpace(req.OrderID)
		if req.OrderID == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "order_id is required"))
			return
		}

		commands, err := requireOrderCommands(svcCtx)
		if err == nil {
			err = commands.ConfirmReceipt(ctx, ports.ConfirmReceiptCommand{
				OrderID: req.OrderID, UserID: identity.UserID, Meta: inventoryRequestMeta(ctx),
			})
		}
		if err != nil {
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		ok(ctx, c, ConfirmReceiptResp{OrderID: req.OrderID, Status: orderstatus.Text(orderstatus.Completed)})
	}
}

func requestUserRefund(ctx context.Context, svcCtx *svc.ServiceContext, req RefundOrderReq, userID int64) error {
	requestID := tracectx.RequestIDFrom(ctx)
	if requestID == "" {
		requestID = req.OrderID + ":refund-request"
	}
	_, err := svcCtx.OrderRpc.RequestRefund(ctx, &orderpb.RequestRefundReq{
		OrderId: req.OrderID, RequesterId: userID, RequesterRole: "user",
		Reason: req.Reason, RequestId: requestID,
	})
	return err
}
