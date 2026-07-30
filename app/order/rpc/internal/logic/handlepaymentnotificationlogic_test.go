package logic

import (
	"encoding/json"
	"testing"

	"flash-mall/app/order/rpc/internal/paymentprovider"
)

func TestBuildPaidCallbackPreservesProviderIdentity(t *testing.T) {
	body, err := buildPaidCallback(paymentprovider.Notification{
		EventID: "notify-100", OutTradeNo: "FM-100", TradeNo: "ALI-100",
		TradeStatus: "TRADE_SUCCESS", AmountFen: 1234,
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["provider"] != paymentprovider.NameAlipaySandbox ||
		payload["provider_trade_no"] != "ALI-100" ||
		payload["trade_status"] != "SUCCESS" ||
		payload["paid_amount_fen"] != float64(1234) {
		t.Fatalf("unexpected callback payload: %#v", payload)
	}
}

func TestIsAlipayPaidStatus(t *testing.T) {
	if !isAlipayPaidStatus("TRADE_SUCCESS") || !isAlipayPaidStatus("TRADE_FINISHED") {
		t.Fatal("successful Alipay statuses were rejected")
	}
	if isAlipayPaidStatus("WAIT_BUYER_PAY") || isAlipayPaidStatus("TRADE_CLOSED") {
		t.Fatal("non-paid Alipay status was accepted as paid")
	}
}
