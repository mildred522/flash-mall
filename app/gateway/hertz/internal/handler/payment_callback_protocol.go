package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"flash-mall/app/common/apperror"
)

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
	body, err := json.Marshal(map[string]any{
		"trade_status": "SUCCESS", "source": "callback", "provider": provider,
		"event_id": eventID, "paid_amount_fen": req.PaidAmountFen,
	})
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func validatePaymentCallbackSignature(secret string, maxSkewSeconds int64, req PaymentCallbackReq) error {
	return validatePaymentCallbackSignatureAt(secret, maxSkewSeconds, req, time.Now())
}

func validatePaymentCallbackSignatureAt(secret string, maxSkewSeconds int64, req PaymentCallbackReq, now time.Time) error {
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
	if skew := now.Sub(time.Unix(timestamp, 0)); skew > time.Duration(maxSkewSeconds)*time.Second || skew < -time.Duration(maxSkewSeconds)*time.Second {
		return apperror.New(apperror.CodeUnauthorized, "payment callback timestamp expired")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(paymentCallbackSigningPayload(req)))
	expected := hex.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(expected), []byte(req.Signature)) != 1 {
		return apperror.New(apperror.CodeUnauthorized, "invalid payment callback signature")
	}
	return nil
}

func paymentCallbackSigningPayload(req PaymentCallbackReq) string {
	return fmt.Sprintf("%s.%s.%s.%s.%s.%d", req.Timestamp, req.Nonce, req.OrderID, req.PaymentOrderID, req.OutTradeNo, req.PaidAmountFen)
}
