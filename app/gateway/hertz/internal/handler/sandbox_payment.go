package handler

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/svc"
	orderpb "flash-mall/app/order/rpc/order"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func SandboxPaymentConfirmHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req SandboxPaymentConfirmReq
		if err := decodeJSONBody(c, &req); err != nil || strings.TrimSpace(req.Token) == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "payment token is required"))
			return
		}
		claims, expired, err := verifyPaymentToken(svcCtx.Config.PaymentCallbackSecret, strings.TrimSpace(req.Token), time.Now())
		if err != nil {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "invalid payment token"))
			return
		}
		if expired {
			fail(ctx, c, consts.StatusConflict, apperror.New(apperror.CodePaymentStatusInvalid, "payment token expired"))
			return
		}
		payment, err := loadPaymentOrderByClaims(ctx, svcCtx, claims)
		if err != nil {
			fail(ctx, c, paymentLookupStatusCode(err), err)
			return
		}
		body, err := json.Marshal(map[string]any{
			"trade_status": "SUCCESS", "source": "sandbox", "provider": "sandbox",
			"event_id": sandboxPaymentEventID(payment.PaymentOrderID), "paid_amount_fen": payment.PayableAmountFen,
		})
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "build sandbox callback failed", err))
			return
		}
		resp, err := markPaymentPaid(ctx, svcCtx, paymentResultInput{
			OrderID: payment.OrderID, PaymentOrderID: payment.PaymentOrderID,
			OutTradeNo: payment.OutTradeNo, CallbackBody: string(body),
		})
		if err != nil {
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		ok(ctx, c, resp)
	}
}

type paymentResultInput struct {
	OrderID        string
	PaymentOrderID string
	OutTradeNo     string
	CallbackBody   string
}

func markPaymentPaid(ctx context.Context, svcCtx *svc.ServiceContext, input paymentResultInput) (PayOrderResp, error) {
	resp, err := svcCtx.OrderRpc.MarkOrderPaid(ctx, &orderpb.MarkOrderPaidReq{
		OrderId: input.OrderID, PaymentOrderId: input.PaymentOrderID,
		OutTradeNo: input.OutTradeNo, CallbackBody: input.CallbackBody,
	})
	if err != nil {
		return PayOrderResp{}, err
	}
	statusText := strings.ToLower(strings.TrimSpace(resp.GetOrderStatus()))
	if statusText == "" {
		statusText = "paid"
	}
	return PayOrderResp{OrderID: input.OrderID, PaymentOrderID: input.PaymentOrderID, OutTradeNo: input.OutTradeNo, Status: statusText}, nil
}

func sandboxPaymentEventID(paymentOrderID string) string {
	return "sandbox:" + paymentOrderID
}
