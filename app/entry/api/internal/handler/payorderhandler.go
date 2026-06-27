package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"flash-mall/app/entry/api/internal/logic"
	"flash-mall/app/entry/api/internal/svc"
	"flash-mall/app/entry/api/internal/types"
	orderclient "flash-mall/app/order/rpc/orderclient"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func PayOrderHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.PayOrderReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		if req.PaymentOrderId != "" || req.OutTradeNo != "" {
			httpx.ErrorCtx(r.Context(), w, errors.New("payment callbacks must use /api/payment/callback"))
			return
		}

		if svcCtx.SessionValidator != nil {
			if err := svcCtx.SessionValidator.Validate(r.Context(), r.Header.Get("Authorization")); err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
		}
		identity, err := extractAuthIdentity(r.Context())
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewPayOrderLogic(r.Context(), svcCtx)
		resp, err := l.PayOrder(&req, identity.UserID)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func PaymentCallbackHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.PayOrderReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		callbackBody, err := buildPaymentCallbackBody(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err := validatePaymentCallbackSignature(svcCtx.Config.PaymentCallbackSecret, svcCtx.Config.PaymentCallbackMaxSkewSeconds, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		resp, err := svcCtx.OrderRpc.MarkOrderPaid(r.Context(), &orderclient.MarkOrderPaidReq{
			OrderId:        req.OrderId,
			PaymentOrderId: req.PaymentOrderId,
			OutTradeNo:     req.OutTradeNo,
			CallbackBody:   callbackBody,
		})
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func buildPaymentCallbackBody(req *types.PayOrderReq) (string, error) {
	if req.OrderId == "" || req.PaymentOrderId == "" || req.OutTradeNo == "" {
		return "", errors.New("order_id, payment_order_id and out_trade_no are required")
	}
	if req.PaidAmountFen <= 0 {
		return "", errors.New("paid_amount_fen is required")
	}

	provider := req.Provider
	if provider == "" {
		provider = "mock"
	}
	eventID := req.EventId
	if eventID == "" {
		eventID = fmt.Sprintf("%s:%s:%s", provider, req.PaymentOrderId, req.OutTradeNo)
	}

	payload := map[string]any{
		"trade_status":    "SUCCESS",
		"source":          "mock",
		"provider":        provider,
		"event_id":        eventID,
		"paid_amount_fen": req.PaidAmountFen,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func validatePaymentCallbackSignature(secret string, maxSkewSeconds int64, req *types.PayOrderReq) error {
	if secret == "" {
		return errors.New("payment callback secret not configured")
	}
	if maxSkewSeconds <= 0 {
		maxSkewSeconds = 300
	}
	if req.Timestamp == "" || req.Nonce == "" || req.Signature == "" {
		return errors.New("payment callback signature fields are required")
	}
	timestamp, err := strconv.ParseInt(req.Timestamp, 10, 64)
	if err != nil {
		return errors.New("invalid payment callback timestamp")
	}
	callbackTime := time.Unix(timestamp, 0)
	maxSkew := time.Duration(maxSkewSeconds) * time.Second
	if time.Since(callbackTime) > maxSkew || time.Until(callbackTime) > maxSkew {
		return errors.New("payment callback timestamp expired")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(paymentCallbackSignPayload(req)))
	expected := hex.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(expected), []byte(req.Signature)) != 1 {
		return errors.New("invalid payment callback signature")
	}
	return nil
}

func paymentCallbackSignPayload(req *types.PayOrderReq) string {
	return fmt.Sprintf("%s.%s.%s.%s.%s.%d", req.Timestamp, req.Nonce, req.OrderId, req.PaymentOrderId, req.OutTradeNo, req.PaidAmountFen)
}
