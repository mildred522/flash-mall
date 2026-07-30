package paymentprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultAlipaySandboxGateway = "https://openapi-sandbox.dl.alipaydev.com/gateway.do"

var alipayTimeZone = time.FixedZone("Asia/Shanghai", 8*60*60)

type Alipay struct {
	config Config
}

func NewAlipay(config Config) (*Alipay, error) {
	config.AppID = strings.TrimSpace(config.AppID)
	config.GatewayURL = strings.TrimSpace(config.GatewayURL)
	config.NotifyURL = strings.TrimSpace(config.NotifyURL)
	if config.GatewayURL == "" {
		config.GatewayURL = defaultAlipaySandboxGateway
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 8 * time.Second}
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.AppID == "" || strings.TrimSpace(config.PrivateKey) == "" ||
		strings.TrimSpace(config.AlipayPublicKey) == "" || config.NotifyURL == "" {
		return nil, errors.New("alipay sandbox requires app id, private key, public key, and notify url")
	}
	if _, err := parsePrivateKey(config.PrivateKey); err != nil {
		return nil, err
	}
	if _, err := parsePublicKey(config.AlipayPublicKey); err != nil {
		return nil, err
	}
	return &Alipay{config: config}, nil
}

func (a *Alipay) Name() string {
	return NameAlipaySandbox
}

func (a *Alipay) Precreate(ctx context.Context, request PrecreateRequest) (PrecreateResult, error) {
	if strings.TrimSpace(request.OutTradeNo) == "" || request.AmountFen <= 0 {
		return PrecreateResult{}, errors.New("out trade number and positive amount are required")
	}
	expireMinutes := request.ExpireMinutes
	if expireMinutes <= 0 {
		expireMinutes = 15
	}
	biz := map[string]any{
		"out_trade_no":    request.OutTradeNo,
		"total_amount":    amountYuan(request.AmountFen),
		"subject":         strings.TrimSpace(request.Subject),
		"timeout_express": fmt.Sprintf("%dm", expireMinutes),
	}
	var response struct {
		Code       string `json:"code"`
		Msg        string `json:"msg"`
		SubCode    string `json:"sub_code"`
		SubMsg     string `json:"sub_msg"`
		OutTradeNo string `json:"out_trade_no"`
		QRCode     string `json:"qr_code"`
	}
	if err := a.call(ctx, "alipay.trade.precreate", biz, "alipay_trade_precreate_response", &response); err != nil {
		return PrecreateResult{}, err
	}
	if response.Code != "10000" || strings.TrimSpace(response.QRCode) == "" {
		return PrecreateResult{}, apiError(response.Code, response.Msg, response.SubCode, response.SubMsg)
	}
	return PrecreateResult{OutTradeNo: response.OutTradeNo, QRCode: response.QRCode}, nil
}

func (a *Alipay) call(ctx context.Context, method string, biz any, responseKey string, target any) error {
	bizJSON, err := json.Marshal(biz)
	if err != nil {
		return fmt.Errorf("marshal alipay request: %w", err)
	}
	values := url.Values{
		"app_id":      {a.config.AppID},
		"method":      {method},
		"format":      {"JSON"},
		"charset":     {"utf-8"},
		"sign_type":   {"RSA2"},
		"timestamp":   {a.config.Now().In(alipayTimeZone).Format("2006-01-02 15:04:05")},
		"version":     {"1.0"},
		"notify_url":  {a.config.NotifyURL},
		"biz_content": {string(bizJSON)},
	}
	signature, err := rsa2Sign(a.config.PrivateKey, canonicalValues(values, "sign"))
	if err != nil {
		return err
	}
	values.Set("sign", signature)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.config.GatewayURL, strings.NewReader(values.Encode()))
	if err != nil {
		return fmt.Errorf("create alipay request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	resp, err := a.config.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("call alipay: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read alipay response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("alipay http status %d", resp.StatusCode)
	}
	return decodeSignedResponse(a.config.AlipayPublicKey, body, responseKey, target)
}

func amountYuan(fen int64) string {
	return strconv.FormatInt(fen/100, 10) + "." + fmt.Sprintf("%02d", fen%100)
}

func apiError(code, msg, subCode, subMsg string) error {
	return fmt.Errorf("alipay api error: code=%s msg=%s sub_code=%s sub_msg=%s", code, msg, subCode, subMsg)
}
