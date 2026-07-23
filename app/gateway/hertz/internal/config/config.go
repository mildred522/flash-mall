package config

import (
	"strings"
	"time"

	gatewaycache "flash-mall/app/gateway/hertz/internal/cache"

	"github.com/zeromicro/go-zero/zrpc"
)

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
	CacheRedisAddr                string `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	CachePrefix                   string `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	CacheEnableL1                 bool   `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	CacheEnableL2                 bool   `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	CacheStaleWhileRevalidate     bool   `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	CacheAllowStaleOnError        bool   `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	CacheL1TTLMillis              int64  `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	CacheSoftTTLSeconds           int64  `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	CacheHardTTLSeconds           int64  `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	CacheMaxEntries               int    `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
}

func (c Config) CacheConfig() gatewaycache.Config {
	l1TTL := time.Duration(c.CacheL1TTLMillis) * time.Millisecond
	if l1TTL <= 0 {
		l1TTL = 2 * time.Second
	}
	softTTL := time.Duration(c.CacheSoftTTLSeconds) * time.Second
	if softTTL <= 0 {
		softTTL = 30 * time.Second
	}
	hardTTL := time.Duration(c.CacheHardTTLSeconds) * time.Second
	if hardTTL <= softTTL {
		hardTTL = 2 * time.Minute
	}
	maxEntries := c.CacheMaxEntries
	if maxEntries <= 0 {
		maxEntries = 512
	}
	return gatewaycache.Config{
		Prefix: c.CachePrefix, EnableL1: c.CacheEnableL1, EnableL2: c.CacheEnableL2,
		StaleWhileRevalidate: c.CacheStaleWhileRevalidate, AllowStaleOnError: c.CacheAllowStaleOnError,
		L1TTL: l1TTL, SoftTTL: softTTL, HardTTL: hardTTL, MaxEntries: maxEntries,
	}
}

func (c Config) NormalizedListenOn() string {
	if c.ListenOn == "" {
		return ":8889"
	}
	return c.ListenOn
}

func (c Config) NormalizedUploadDir() string {
	if dir := strings.TrimSpace(c.UploadDir); dir != "" {
		return dir
	}
	return ".runtime/uploads"
}
