package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/common/orderstatus"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func PaymentCallbackHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req PaymentCallbackReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid payment callback request"))
			return
		}
		if err := validatePaymentCallbackSignature(svcCtx.Config.PaymentCallbackSecret, svcCtx.Config.PaymentCallbackMaxSkewSeconds, req); err != nil {
			fail(ctx, c, consts.StatusUnauthorized, err)
			return
		}
		callbackBody, err := paymentCallbackBody(req)
		if err != nil {
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
		ok(ctx, c, resp)
	}
}

func OrderStatusPollHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, found := authctx.IdentityFrom(ctx)
		if !found || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "user login required"))
			return
		}
		requestID := strings.TrimSpace(c.Query("request_id"))
		if requestID == "" {
			ok(ctx, c, OrderStatusPollResp{Status: "missing_request_id"})
			return
		}
		db, err := orderDB(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		var orderID string
		var statusCode int64
		err = db.QueryRowContext(ctx, "SELECT id, status FROM orders WHERE request_id = ? AND user_id = ? LIMIT 1", requestID, identity.UserID).Scan(&orderID, &statusCode)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				ok(ctx, c, OrderStatusPollResp{RequestID: requestID, Status: "processing"})
				return
			}
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order status lookup failed", err))
			return
		}
		ok(ctx, c, OrderStatusPollResp{RequestID: requestID, OrderID: orderID, Status: orderstatus.Text(statusCode)})
	}
}

func paymentCallbackBody(req PaymentCallbackReq) (string, error) {
	if strings.TrimSpace(req.OrderID) == "" || strings.TrimSpace(req.PaymentOrderID) == "" || strings.TrimSpace(req.OutTradeNo) == "" || req.PaidAmountFen <= 0 {
		return "", apperror.New(apperror.CodeInvalidArgument, "order_id, payment_order_id, out_trade_no and paid_amount_fen are required")
	}
	provider := strings.TrimSpace(req.Provider)
	if provider == "" {
		provider = "mock"
	}
	eventID := strings.TrimSpace(req.EventID)
	if eventID == "" {
		eventID = fmt.Sprintf("%s:%s:%s", provider, req.PaymentOrderID, req.OutTradeNo)
	}
	body, err := json.Marshal(map[string]any{"trade_status": "SUCCESS", "source": "callback", "provider": provider, "event_id": eventID, "paid_amount_fen": req.PaidAmountFen})
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func validatePaymentCallbackSignature(secret string, maxSkewSeconds int64, req PaymentCallbackReq) error {
	if strings.TrimSpace(secret) == "" {
		return apperror.New(apperror.CodeUnauthorized, "payment callback secret not configured")
	}
	if maxSkewSeconds <= 0 {
		maxSkewSeconds = 300
	}
	if strings.TrimSpace(req.Timestamp) == "" || strings.TrimSpace(req.Nonce) == "" || strings.TrimSpace(req.Signature) == "" {
		return apperror.New(apperror.CodeUnauthorized, "payment callback signature fields are required")
	}
	timestamp, err := strconv.ParseInt(req.Timestamp, 10, 64)
	if err != nil {
		return apperror.New(apperror.CodeUnauthorized, "invalid payment callback timestamp")
	}
	if skew := time.Since(time.Unix(timestamp, 0)); skew > time.Duration(maxSkewSeconds)*time.Second || skew < -time.Duration(maxSkewSeconds)*time.Second {
		return apperror.New(apperror.CodeUnauthorized, "payment callback timestamp expired")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(fmt.Sprintf("%s.%s.%s.%s.%s.%d", req.Timestamp, req.Nonce, req.OrderID, req.PaymentOrderID, req.OutTradeNo, req.PaidAmountFen)))
	expected := hex.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(expected), []byte(req.Signature)) != 1 {
		return apperror.New(apperror.CodeUnauthorized, "invalid payment callback signature")
	}
	return nil
}
