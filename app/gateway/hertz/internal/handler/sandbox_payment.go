package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/common/paymentstatus"
	"flash-mall/app/gateway/hertz/internal/svc"
	orderpb "flash-mall/app/order/rpc/order"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

const defaultSandboxPaymentTokenTTL = 15 * time.Minute

func buildPaymentIntentResp(c *app.RequestContext, svcCtx *svc.ServiceContext, payment userPaymentOrder, now time.Time) (PayOrderResp, error) {
	statusText := sandboxPaymentStatus(payment.PaymentStatus, false)
	resp := PayOrderResp{
		OrderID:          payment.OrderID,
		PaymentOrderID:   payment.PaymentOrderID,
		OutTradeNo:       payment.OutTradeNo,
		PayableAmountFen: payment.PayableAmountFen,
		Status:           statusText,
	}
	if statusText == "paid" {
		return resp, nil
	}
	ttl := time.Duration(svcCtx.Config.SandboxPaymentTokenTTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = defaultSandboxPaymentTokenTTL
	}
	claims := paymentTokenClaims{
		OrderID: payment.OrderID, PaymentOrderID: payment.PaymentOrderID, OutTradeNo: payment.OutTradeNo,
		PayableAmountFen: payment.PayableAmountFen, ExpiresAt: now.Add(ttl).Unix(),
	}
	token, err := signPaymentToken(svcCtx.Config.PaymentCallbackSecret, claims)
	if err != nil {
		return PayOrderResp{}, apperror.Wrap(apperror.CodeInternal, "create payment token failed", err)
	}
	resp.ExpiresAt = claims.ExpiresAt
	resp.QRURL = paymentPublicBaseURL(c) + "/pay?token=" + url.QueryEscape(token)
	return resp, nil
}

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
		}
		if err != nil {
			fail(ctx, c, paymentLookupStatusCode(err), err)
			return
		}
		ok(ctx, c, paymentStatusResponse(payment, expiresAt, expired))
	}
}

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

func loadUserPaymentOrderByID(ctx context.Context, svcCtx *svc.ServiceContext, paymentOrderID string, userID int64) (userPaymentOrder, error) {
	return queryPaymentOrder(ctx, svcCtx, `WHERE p.id = ? AND o.user_id = ? LIMIT 1`, paymentOrderID, userID)
}

func loadPaymentOrderByClaims(ctx context.Context, svcCtx *svc.ServiceContext, claims paymentTokenClaims) (userPaymentOrder, error) {
	payment, err := queryPaymentOrder(ctx, svcCtx, `WHERE p.id = ? AND p.order_id = ? AND p.out_trade_no = ? LIMIT 1`, claims.PaymentOrderID, claims.OrderID, claims.OutTradeNo)
	if err != nil {
		return userPaymentOrder{}, err
	}
	if !paymentMatchesClaims(payment, claims) {
		return userPaymentOrder{}, apperror.New(apperror.CodePaymentStatusInvalid, "payment token does not match payment order")
	}
	return payment, nil
}

func queryPaymentOrder(ctx context.Context, svcCtx *svc.ServiceContext, where string, args ...any) (userPaymentOrder, error) {
	db, err := orderDB(svcCtx)
	if err != nil {
		return userPaymentOrder{}, err
	}
	var payment userPaymentOrder
	err = db.QueryRowContext(ctx, `
SELECT o.id, o.user_id, o.status, p.id, p.status, p.out_trade_no, p.payable_amount_fen
FROM orders o
JOIN payment_order p ON p.order_id = o.id
`+where, args...).Scan(
		&payment.OrderID, &payment.UserID, &payment.OrderStatus,
		&payment.PaymentOrderID, &payment.PaymentStatus, &payment.OutTradeNo, &payment.PayableAmountFen,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return userPaymentOrder{}, apperror.New(apperror.CodeNotFound, "payment order not found")
		}
		return userPaymentOrder{}, err
	}
	return payment, nil
}

func paymentMatchesClaims(payment userPaymentOrder, claims paymentTokenClaims) bool {
	return payment.OrderID == claims.OrderID && payment.PaymentOrderID == claims.PaymentOrderID &&
		payment.OutTradeNo == claims.OutTradeNo && payment.PayableAmountFen == claims.PayableAmountFen
}

func paymentStatusResponse(payment userPaymentOrder, expiresAt int64, expired bool) PaymentStatusResp {
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

func sandboxPaymentEventID(paymentOrderID string) string {
	return "sandbox:" + paymentOrderID
}

func paymentPublicBaseURL(c *app.RequestContext) string {
	scheme := strings.TrimSpace(strings.Split(string(c.GetHeader("X-Forwarded-Proto")), ",")[0])
	if scheme != "https" {
		scheme = "http"
	}
	return scheme + "://" + strings.TrimSpace(string(c.Host()))
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
