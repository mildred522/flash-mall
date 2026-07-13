package config

import "github.com/zeromicro/go-zero/zrpc"

type Config struct {
	Name                          string
	ListenOn                      string
	DataSource                    string
	OrderDataSource               string
	AuthDataSource                string
	JwtAuthSecret                 string
	AuthServiceBaseURL            string
	PaymentCallbackSecret         string
	PaymentCallbackMaxSkewSeconds int64 `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	SandboxPaymentTokenTTLSeconds int64 `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	InventoryKitexEndpoint        string
	DtmServer                     string
	OrderRpcTarget                string
	DtmTimeoutToFailSeconds       int64 `json:",optional"`      //nolint:staticcheck // go-zero config uses optional in json tags.
	DtmRequestTimeoutSeconds      int64 `json:",optional"`      //nolint:staticcheck // go-zero config uses optional in json tags.
	DtmWaitResult                 bool  `json:",optional"`      //nolint:staticcheck // go-zero config uses optional in json tags.
	EnableLiveStockOverlay        bool  `json:",default=false"` //nolint:staticcheck // go-zero config uses default in json tags.
	ProductRpcConf                zrpc.RpcClientConf
	OrderRpcConf                  zrpc.RpcClientConf
	UploadDir                     string `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
}

func (c Config) NormalizedListenOn() string {
	if c.ListenOn == "" {
		return ":8889"
	}
	return c.ListenOn
}
