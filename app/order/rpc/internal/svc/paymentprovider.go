package svc

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"flash-mall/app/order/rpc/internal/config"
	"flash-mall/app/order/rpc/internal/paymentprovider"
)

func buildPaymentProvider(c config.Config) (paymentprovider.Provider, error) {
	mode := strings.ToLower(strings.TrimSpace(c.PaymentProvider))
	if mode == "" || mode == paymentprovider.NameLocalSandbox {
		return nil, nil
	}
	if mode != paymentprovider.NameAlipaySandbox {
		return nil, fmt.Errorf("unsupported payment provider %q", c.PaymentProvider)
	}
	timeout := time.Duration(c.AlipaySandbox.RequestTimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	return paymentprovider.NewAlipay(paymentprovider.Config{
		AppID: c.AlipaySandbox.AppID, GatewayURL: c.AlipaySandbox.GatewayURL,
		PrivateKey: c.AlipaySandbox.PrivateKey, AlipayPublicKey: c.AlipaySandbox.AlipayPublicKey,
		NotifyURL: c.AlipaySandbox.NotifyURL, HTTPClient: &http.Client{Timeout: timeout},
	})
}
