package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"
)

func TestValidatePaymentCallbackSignatureAtAcceptsCanonicalPayload(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	req := PaymentCallbackReq{
		Timestamp: fmt.Sprint(now.Unix()), Nonce: "nonce-1", OrderID: "order-1",
		PaymentOrderID: "pay-1", OutTradeNo: "trade-1", PaidAmountFen: 9900,
	}
	mac := hmac.New(sha256.New, []byte("secret"))
	_, _ = mac.Write([]byte(fmt.Sprintf("%s.%s.%s.%s.%s.%d", req.Timestamp, req.Nonce, req.OrderID, req.PaymentOrderID, req.OutTradeNo, req.PaidAmountFen)))
	req.Signature = hex.EncodeToString(mac.Sum(nil))

	if err := validatePaymentCallbackSignatureAt("secret", 300, req, now); err != nil {
		t.Fatal(err)
	}
}

func TestValidatePaymentCallbackSignatureAtRejectsExpiredTimestamp(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	req := PaymentCallbackReq{Timestamp: fmt.Sprint(now.Add(-6 * time.Minute).Unix()), Nonce: "nonce-1", Signature: "ignored"}
	if err := validatePaymentCallbackSignatureAt("secret", 300, req, now); err == nil {
		t.Fatal("expired callback timestamp must be rejected")
	}
}
