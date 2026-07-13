package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type paymentTokenClaims struct {
	OrderID          string `json:"order_id"`
	PaymentOrderID   string `json:"payment_order_id"`
	OutTradeNo       string `json:"out_trade_no"`
	PayableAmountFen int64  `json:"payable_amount_fen"`
	ExpiresAt        int64  `json:"expires_at"`
}

func signPaymentToken(secret string, claims paymentTokenClaims) (string, error) {
	if err := validatePaymentTokenClaims(secret, claims); err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(encoded))
	return encoded + "." + hex.EncodeToString(mac.Sum(nil)), nil
}

func verifyPaymentToken(secret, token string, now time.Time) (paymentTokenClaims, bool, error) {
	if strings.TrimSpace(secret) == "" {
		return paymentTokenClaims{}, false, errors.New("payment token secret is required")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return paymentTokenClaims{}, false, errors.New("invalid payment token")
	}
	provided, err := hex.DecodeString(parts[1])
	if err != nil {
		return paymentTokenClaims{}, false, errors.New("invalid payment token signature")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(parts[0]))
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return paymentTokenClaims{}, false, errors.New("invalid payment token signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return paymentTokenClaims{}, false, errors.New("invalid payment token payload")
	}
	var claims paymentTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return paymentTokenClaims{}, false, errors.New("invalid payment token payload")
	}
	if err := validatePaymentTokenClaims(secret, claims); err != nil {
		return paymentTokenClaims{}, false, err
	}
	return claims, now.Unix() > claims.ExpiresAt, nil
}

func validatePaymentTokenClaims(secret string, claims paymentTokenClaims) error {
	if strings.TrimSpace(secret) == "" {
		return errors.New("payment token secret is required")
	}
	if strings.TrimSpace(claims.OrderID) == "" || strings.TrimSpace(claims.PaymentOrderID) == "" || strings.TrimSpace(claims.OutTradeNo) == "" {
		return errors.New("payment token identifiers are required")
	}
	if claims.PayableAmountFen <= 0 || claims.ExpiresAt <= 0 {
		return errors.New("payment token amount and expiry are required")
	}
	return nil
}
