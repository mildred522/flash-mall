package handler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/paymentstatus"
	"flash-mall/app/gateway/hertz/internal/config"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestPaymentMatchesTokenClaims(t *testing.T) {
	payment := userPaymentOrder{OrderID: "o-1", PaymentOrderID: "p-1", OutTradeNo: "t-1", PayableAmountFen: 9900}
	claims := paymentTokenClaims{OrderID: "o-1", PaymentOrderID: "p-1", OutTradeNo: "t-1", PayableAmountFen: 9900, ExpiresAt: time.Now().Add(time.Minute).Unix()}
	if !paymentMatchesClaims(payment, claims) {
		t.Fatal("matching payment and claims should be accepted")
	}
	claims.PayableAmountFen++
	if paymentMatchesClaims(payment, claims) {
		t.Fatal("amount mismatch should be rejected")
	}
}

func TestSandboxPaymentEnforcesOwnerAndTokenBinding(t *testing.T) {
	const dsn = "root:6494kj06@tcp(127.0.0.1:3307)/mall_order?charset=utf8mb4&parseTime=true&loc=Local"
	conn := sqlx.NewMysql(dsn)
	svcCtx := &svc.ServiceContext{OrderSqlConn: conn}
	unique := time.Now().UnixNano()
	orderID := fmt.Sprintf("sandbox-test-%d", unique)
	paymentID := "pay:" + orderID
	tradeNo := "sandbox-" + orderID
	ctx := context.Background()

	if _, err := conn.ExecCtx(ctx, "INSERT INTO orders (id, request_id, user_id, product_id, amount, status) VALUES (?, ?, 7001, 100, 1, 0)", orderID, "req-"+orderID); err != nil {
		t.Fatalf("seed order: %v", err)
	}
	t.Cleanup(func() {
		_, _ = conn.ExecCtx(ctx, "DELETE FROM payment_callback_event WHERE order_id = ?", orderID)
		_, _ = conn.ExecCtx(ctx, "DELETE FROM payment_order WHERE order_id = ?", orderID)
		_, _ = conn.ExecCtx(ctx, "DELETE FROM orders WHERE id = ?", orderID)
	})
	if _, err := conn.ExecCtx(ctx, "INSERT INTO payment_order (id, order_id, user_id, payable_amount_fen, status, out_trade_no) VALUES (?, ?, 7001, 9900, 0, ?)", paymentID, orderID, tradeNo); err != nil {
		t.Fatalf("seed payment: %v", err)
	}

	first, err := loadUserPaymentOrder(ctx, svcCtx, orderID, 7001)
	if err != nil {
		t.Fatalf("owner should load payment: %v", err)
	}
	second, err := loadUserPaymentOrder(ctx, svcCtx, orderID, 7001)
	if err != nil || first.PaymentOrderID != second.PaymentOrderID || first.OutTradeNo != second.OutTradeNo {
		t.Fatalf("repeated intent lookup must be stable: first=%#v second=%#v err=%v", first, second, err)
	}
	if _, err := loadUserPaymentOrder(ctx, svcCtx, orderID, 7002); apperror.CodeOf(err) != apperror.CodeOrderNotFound {
		t.Fatalf("different user should not load payment: %v", err)
	}

	claims := paymentTokenClaims{OrderID: orderID, PaymentOrderID: paymentID, OutTradeNo: tradeNo, PayableAmountFen: 9900, ExpiresAt: time.Now().Add(time.Minute).Unix()}
	if _, err := loadPaymentOrderByClaims(ctx, svcCtx, claims); err != nil {
		t.Fatalf("matching token binding should load: %v", err)
	}
	claims.PayableAmountFen++
	if _, err := loadPaymentOrderByClaims(ctx, svcCtx, claims); apperror.CodeOf(err) != apperror.CodePaymentStatusInvalid {
		t.Fatalf("amount mismatch should be rejected: %v", err)
	}
}

func TestPaymentStatusRejectsTamperedTokenAsUnauthorized(t *testing.T) {
	h := server.Default()
	registerAuthRoutes(h, &svc.ServiceContext{Config: config.Config{
		JwtAuthSecret:         "jwt-secret",
		PaymentCallbackSecret: "payment-secret",
	}})

	resp := ut.PerformRequest(h.Engine, "GET", "/api/payment/status?token=tampered", nil).Result()
	if resp.StatusCode() != consts.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", resp.StatusCode(), resp.Body())
	}
}

func TestSandboxPaymentStatus(t *testing.T) {
	if got := sandboxPaymentStatus(paymentstatus.Init, false); got != "pending" {
		t.Fatalf("init status=%q", got)
	}
	if got := sandboxPaymentStatus(paymentstatus.Init, true); got != "expired" {
		t.Fatalf("expired status=%q", got)
	}
	if got := sandboxPaymentStatus(paymentstatus.Success, true); got != "paid" {
		t.Fatalf("paid status must win over token expiry, got=%q", got)
	}
}

func TestSandboxEventIDIsStable(t *testing.T) {
	if got := sandboxPaymentEventID("pay:o-1"); got != "sandbox:pay:o-1" {
		t.Fatalf("event id=%q", got)
	}
}
