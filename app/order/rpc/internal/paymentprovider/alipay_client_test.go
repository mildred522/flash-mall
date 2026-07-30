package paymentprovider

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestAmountYuanUsesExactFenPrecision(t *testing.T) {
	for _, test := range []struct {
		fen  int64
		want string
	}{{1, "0.01"}, {100, "1.00"}, {1234, "12.34"}} {
		if got := amountYuan(test.fen); got != test.want {
			t.Fatalf("amountYuan(%d) = %q, want %q", test.fen, got, test.want)
		}
	}
}

func TestAlipayPrecreateReturnsVerifiedQRCode(t *testing.T) {
	privateKey, publicKey := testRSAKeyPair(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		values, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatal(err)
		}
		if values.Get("method") != "alipay.trade.precreate" {
			t.Fatalf("method = %q", values.Get("method"))
		}
		if values.Get("timestamp") != "2026-07-30 18:00:00" {
			t.Fatalf("timestamp = %q, want Asia/Shanghai time", values.Get("timestamp"))
		}
		var biz map[string]any
		if err := json.Unmarshal([]byte(values.Get("biz_content")), &biz); err != nil {
			t.Fatal(err)
		}
		if biz["out_trade_no"] != "FM-100" || biz["total_amount"] != "12.34" {
			t.Fatalf("unexpected biz_content: %#v", biz)
		}
		response := json.RawMessage(`{"code":"10000","msg":"Success","out_trade_no":"FM-100","qr_code":"https://qr.alipay.test/100"}`)
		signature, err := rsa2Sign(privateKey, string(response))
		if err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"alipay_trade_precreate_response": response,
			"sign":                            signature,
		})
	}))
	defer server.Close()

	client, err := NewAlipay(Config{
		AppID: "sandbox-app", GatewayURL: server.URL,
		PrivateKey: privateKey, AlipayPublicKey: publicKey,
		NotifyURL: "https://mall.test/api/payment/alipay/notify",
		Now:       func() time.Time { return time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("NewAlipay() error = %v", err)
	}
	result, err := client.Precreate(context.Background(), PrecreateRequest{
		OutTradeNo: "FM-100", Subject: "Flash Mall order FM-100",
		AmountFen: 1234, ExpireMinutes: 15,
	})
	if err != nil {
		t.Fatalf("Precreate() error = %v", err)
	}
	if result.QRCode != "https://qr.alipay.test/100" {
		t.Fatalf("QRCode = %q", result.QRCode)
	}
}

func TestAlipayVerifyNotificationChecksAppAndAmount(t *testing.T) {
	privateKey, publicKey := testRSAKeyPair(t)
	client, err := NewAlipay(Config{
		AppID: "sandbox-app", GatewayURL: "https://sandbox.invalid",
		PrivateKey: privateKey, AlipayPublicKey: publicKey,
		NotifyURL: "https://mall.test/api/payment/alipay/notify",
	})
	if err != nil {
		t.Fatal(err)
	}
	values := url.Values{
		"app_id":       {"sandbox-app"},
		"notify_id":    {"notify-100"},
		"out_trade_no": {"FM-100"},
		"trade_no":     {"202607300001"},
		"trade_status": {"TRADE_SUCCESS"},
		"total_amount": {"12.34"},
		"sign_type":    {"RSA2"},
	}
	signature, err := rsa2Sign(privateKey, canonicalValues(values, "sign", "sign_type"))
	if err != nil {
		t.Fatal(err)
	}
	values.Set("sign", signature)

	notification, err := client.VerifyNotification(values)
	if err != nil {
		t.Fatalf("VerifyNotification() error = %v", err)
	}
	if notification.AmountFen != 1234 || notification.EventID != "notify-100" {
		t.Fatalf("unexpected notification: %#v", notification)
	}
	values.Set("app_id", "another-app")
	signature, err = rsa2Sign(privateKey, canonicalValues(values, "sign", "sign_type"))
	if err != nil {
		t.Fatal(err)
	}
	values.Set("sign", signature)
	if _, err := client.VerifyNotification(values); err == nil {
		t.Fatal("VerifyNotification() accepted another app id")
	}
}

func TestAlipayQueryCloseAndRefund(t *testing.T) {
	privateKey, publicKey := testRSAKeyPair(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		method := r.Form.Get("method")
		var key string
		var response json.RawMessage
		switch method {
		case "alipay.trade.query":
			key = "alipay_trade_query_response"
			response = json.RawMessage(`{"code":"10000","msg":"Success","out_trade_no":"FM-100","trade_no":"ALI-100","trade_status":"TRADE_SUCCESS","total_amount":"12.34"}`)
		case "alipay.trade.close":
			key = "alipay_trade_close_response"
			response = json.RawMessage(`{"code":"10000","msg":"Success","out_trade_no":"FM-100"}`)
		case "alipay.trade.refund":
			key = "alipay_trade_refund_response"
			response = json.RawMessage(`{"code":"10000","msg":"Success","out_trade_no":"FM-100","trade_no":"ALI-100","refund_fee":"12.34"}`)
		default:
			t.Fatalf("unexpected method %q", method)
		}
		signature, err := rsa2Sign(privateKey, string(response))
		if err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{key: response, "sign": signature})
	}))
	defer server.Close()
	client, err := NewAlipay(Config{
		AppID: "sandbox-app", GatewayURL: server.URL,
		PrivateKey: privateKey, AlipayPublicKey: publicKey,
		NotifyURL: "https://mall.test/api/payment/alipay/notify",
	})
	if err != nil {
		t.Fatal(err)
	}
	query, err := client.Query(context.Background(), "FM-100")
	if err != nil || query.Status != "TRADE_SUCCESS" || query.AmountFen != 1234 {
		t.Fatalf("Query() = %#v, %v", query, err)
	}
	if err := client.Close(context.Background(), "FM-100"); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	refund, err := client.Refund(context.Background(), RefundRequest{
		OutTradeNo: "FM-100", RefundID: "RF-100", AmountFen: 1234, Reason: "approved",
	})
	if err != nil || refund.Status != "SUCCESS" {
		t.Fatalf("Refund() = %#v, %v", refund, err)
	}
}

func testRSAKeyPair(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})),
		string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))
}
