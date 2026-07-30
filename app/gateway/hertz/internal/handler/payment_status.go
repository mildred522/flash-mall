package handler

import (
	"context"
	"strings"
	"time"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/common/paymentstatus"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func PaymentStatusHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		token := strings.TrimSpace(c.Query("token"))
		paymentOrderID := strings.TrimSpace(c.Query("payment_order_id"))
		if (token == "") == (paymentOrderID == "") {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "provide exactly one of token or payment_order_id"))
			return
		}

		var payment userPaymentOrder
		var expiresAt int64
		var expired bool
		var err error
		if token != "" {
			var claims paymentTokenClaims
			claims, expired, err = verifyPaymentToken(svcCtx.Config.PaymentCallbackSecret, token, time.Now())
			if err != nil {
				fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "invalid payment token"))
				return
			}
			expiresAt = claims.ExpiresAt
			payment, err = loadPaymentOrderByClaims(ctx, svcCtx, claims)
		} else {
			identity, found := authctx.IdentityFrom(ctx)
			if !found || identity.UserID <= 0 {
				fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "user login or payment token required"))
				return
			}
			payment, err = loadUserPaymentOrderByID(ctx, svcCtx, paymentOrderID, identity.UserID)
			if err == nil {
				expiresAt = payment.ExpiresAt
				expired = expiresAt > 0 && time.Now().Unix() >= expiresAt
			}
		}
		if err != nil {
			fail(ctx, c, paymentLookupStatusCode(err), err)
			return
		}
		ok(ctx, c, paymentStatusResponse(payment, expiresAt, expired))
	}
}

func loadUserPaymentOrderByID(ctx context.Context, svcCtx *svc.ServiceContext, paymentOrderID string, userID int64) (userPaymentOrder, error) {
	service, err := orderQueryService(svcCtx)
	if err != nil {
		return userPaymentOrder{}, apperror.Wrap(apperror.CodeInternal, "order query service unavailable", err)
	}
	return service.PaymentByID(ctx, paymentOrderID, userID)
}

func loadPaymentOrderByClaims(ctx context.Context, svcCtx *svc.ServiceContext, claims paymentTokenClaims) (userPaymentOrder, error) {
	service, err := orderQueryService(svcCtx)
	if err != nil {
		return userPaymentOrder{}, apperror.Wrap(apperror.CodeInternal, "order query service unavailable", err)
	}
	payment, err := service.PaymentByClaims(ctx, claims.PaymentOrderID, claims.OrderID, claims.OutTradeNo)
	if err != nil {
		return userPaymentOrder{}, err
	}
	if !paymentMatchesClaims(payment, claims) {
		return userPaymentOrder{}, apperror.New(apperror.CodePaymentStatusInvalid, "payment token does not match payment order")
	}
	return payment, nil
}

func paymentMatchesClaims(payment userPaymentOrder, claims paymentTokenClaims) bool {
	return payment.OrderID == claims.OrderID && payment.PaymentOrderID == claims.PaymentOrderID &&
		payment.OutTradeNo == claims.OutTradeNo && payment.PayableAmountFen == claims.PayableAmountFen
}

func paymentStatusResponse(payment userPaymentOrder, expiresAt int64, expired bool) PaymentStatusResp {
	if expiresAt <= 0 {
		expiresAt = payment.ExpiresAt
	}
	return PaymentStatusResp{
		OrderID: payment.OrderID, PaymentOrderID: payment.PaymentOrderID, OutTradeNo: payment.OutTradeNo,
		PayableAmountFen: payment.PayableAmountFen, Status: sandboxPaymentStatus(payment.PaymentStatus, expired), ExpiresAt: expiresAt,
	}
}

func sandboxPaymentStatus(status int64, expired bool) string {
	if status == paymentstatus.Success {
		return "paid"
	}
	if expired && status == paymentstatus.Init {
		return "expired"
	}
	switch status {
	case paymentstatus.Init:
		return "pending"
	case paymentstatus.Closed:
		return "closed"
	case paymentstatus.Failed:
		return "failed"
	default:
		return "unknown"
	}
}

func paymentLookupStatusCode(err error) int {
	switch apperror.CodeOf(err) {
	case apperror.CodeUnauthorized:
		return consts.StatusUnauthorized
	case apperror.CodeNotFound, apperror.CodeOrderNotFound:
		return consts.StatusNotFound
	case apperror.CodePaymentStatusInvalid:
		return consts.StatusConflict
	default:
		return consts.StatusBadGateway
	}
}
