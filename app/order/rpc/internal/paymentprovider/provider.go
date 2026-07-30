package paymentprovider

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

const (
	NameLocalSandbox  = "local_sandbox"
	NameAlipaySandbox = "alipay_sandbox"
)

type Config struct {
	AppID           string
	GatewayURL      string
	PrivateKey      string
	AlipayPublicKey string
	NotifyURL       string
	HTTPClient      *http.Client
	Now             func() time.Time
}

type PrecreateRequest struct {
	OutTradeNo    string
	Subject       string
	AmountFen     int64
	ExpireMinutes int
}

type PrecreateResult struct {
	OutTradeNo string
	QRCode     string
}

type QueryResult struct {
	OutTradeNo string
	TradeNo    string
	Status     string
	AmountFen  int64
}

type RefundRequest struct {
	OutTradeNo string
	RefundID   string
	AmountFen  int64
	Reason     string
}

type RefundResult struct {
	OutTradeNo string
	TradeNo    string
	RefundID   string
	Status     string
}

type Notification struct {
	AppID       string
	EventID     string
	OutTradeNo  string
	TradeNo     string
	TradeStatus string
	AmountFen   int64
	Raw         string
}

type Provider interface {
	Name() string
	Precreate(context.Context, PrecreateRequest) (PrecreateResult, error)
	Query(context.Context, string) (QueryResult, error)
	Close(context.Context, string) error
	Refund(context.Context, RefundRequest) (RefundResult, error)
	VerifyNotification(url.Values) (Notification, error)
}
