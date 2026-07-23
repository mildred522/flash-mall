package handler

import (
	"context"
	"time"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func PaymentCallbackHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		started := time.Now()
		result := "error"
		defer func() { recordPaymentCallback(result, time.Since(started)) }()
		var req PaymentCallbackReq
		if err := decodeJSONBody(c, &req); err != nil {
			result = "invalid_request"
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid payment callback request"))
			return
		}
		if err := validatePaymentCallbackSignature(svcCtx.Config.PaymentCallbackSecret, svcCtx.Config.PaymentCallbackMaxSkewSeconds, req); err != nil {
			result = "unauthorized"
			fail(ctx, c, consts.StatusUnauthorized, err)
			return
		}
		callbackBody, err := paymentCallbackBody(req)
		if err != nil {
			result = "invalid_payload"
			fail(ctx, c, consts.StatusBadRequest, err)
			return
		}
		resp, err := markPaymentPaid(ctx, svcCtx, paymentResultInput{
			OrderID: req.OrderID, PaymentOrderID: req.PaymentOrderID,
			OutTradeNo: req.OutTradeNo, CallbackBody: callbackBody,
		})
		if err != nil {
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		result = "success"
		ok(ctx, c, resp)
	}
}
