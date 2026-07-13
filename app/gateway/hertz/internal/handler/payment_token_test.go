package handler

import (
	"strings"
	"testing"
	"time"
)

func TestPaymentTokenRoundTrip(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	want := paymentTokenClaims{
		OrderID: "order-1", PaymentOrderID: "pay:order-1", OutTradeNo: "sandbox-order-1",
		PayableAmountFen: 9900, ExpiresAt: now.Add(15 * time.Minute).Unix(),
	}
	token, err := signPaymentToken("secret", want)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	got, expired, err := verifyPaymentToken("secret", token, now)
	if err != nil || expired {
		t.Fatalf("verify token: expired=%v err=%v", expired, err)
	}
	if got != want {
		t.Fatalf("claims = %#v, want %#v", got, want)
	}
}

func TestPaymentTokenRejectsTampering(t *testing.T) {
	claims := paymentTokenClaims{OrderID: "order-1", PaymentOrderID: "pay:order-1", OutTradeNo: "trade-1", PayableAmountFen: 9900, ExpiresAt: time.Now().Add(time.Minute).Unix()}
	token, err := signPaymentToken("secret", claims)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	parts[0] = parts[0][:len(parts[0])-1] + "A"
	if _, _, err := verifyPaymentToken("secret", strings.Join(parts, "."), time.Now()); err == nil {
		t.Fatal("tampered payment token should fail")
	}
}

func TestPaymentTokenReportsSignedExpiry(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	claims := paymentTokenClaims{OrderID: "order-1", PaymentOrderID: "pay:order-1", OutTradeNo: "trade-1", PayableAmountFen: 9900, ExpiresAt: now.Add(-time.Second).Unix()}
	token, err := signPaymentToken("secret", claims)
	if err != nil {
		t.Fatal(err)
	}
	got, expired, err := verifyPaymentToken("secret", token, now)
	if err != nil {
		t.Fatalf("signed expired token should still decode: %v", err)
	}
	if !expired || got != claims {
		t.Fatalf("claims=%#v expired=%v", got, expired)
	}
}

func TestPaymentTokenRequiresCompleteClaims(t *testing.T) {
	if _, err := signPaymentToken("secret", paymentTokenClaims{}); err == nil {
		t.Fatal("incomplete claims should fail")
	}
	if _, err := signPaymentToken("", paymentTokenClaims{OrderID: "o", PaymentOrderID: "p", OutTradeNo: "t", PayableAmountFen: 1, ExpiresAt: 1}); err == nil {
		t.Fatal("empty secret should fail")
	}
}
