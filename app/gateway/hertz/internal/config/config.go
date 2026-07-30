package config

import (
	"strings"
	"time"

	commonobs "flash-mall/app/common/observability"
	gatewaycache "flash-mall/app/gateway/hertz/internal/cache"
	"flash-mall/app/gateway/hertz/internal/existencefilter"

	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	Name                                    string
	ListenOn                                string
	DataSource                              string
	OrderDataSource                         string
	AuthDataSource                          string
	JwtAuthSecret                           string
	AuthServiceBaseURL                      string
	PaymentCallbackSecret                   string
	PaymentCallbackMaxSkewSeconds           int64 `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	SandboxPaymentTokenTTLSeconds           int64 `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	InventoryKitexEndpoint                  string
	DtmServer                               string
	OrderRpcTarget                          string
	DtmTimeoutToFailSeconds                 int64 `json:",optional"`      //nolint:staticcheck // go-zero config uses optional in json tags.
	DtmRequestTimeoutSeconds                int64 `json:",optional"`      //nolint:staticcheck // go-zero config uses optional in json tags.
	DtmWaitResult                           bool  `json:",optional"`      //nolint:staticcheck // go-zero config uses optional in json tags.
	EnableLiveStockOverlay                  bool  `json:",default=false"` //nolint:staticcheck // go-zero config uses default in json tags.
	ProductRpcConf                          zrpc.RpcClientConf
	OrderRpcConf                            zrpc.RpcClientConf
	Observability                           commonobs.Config `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	UploadDir                               string           `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	CacheRedisAddr                          string           `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	CachePrefix                             string           `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	CacheEnableL1                           bool             `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	CacheEnableL2                           bool             `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	CacheStaleWhileRevalidate               bool             `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	CacheAllowStaleOnError                  bool             `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	CacheL1TTLMillis                        int64            `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	CacheSoftTTLSeconds                     int64            `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	CacheHardTTLSeconds                     int64            `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	CacheMaxEntries                         int              `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	ProductExistenceFilterEnabled           bool             `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	ProductExistenceFilterPrefix            string           `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	ProductExistenceFilterExpectedItems     uint64           `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	ProductExistenceFilterFalsePositiveRate float64          `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	ProductExistenceFilterRebuildHours      int64            `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	ProductExistenceFilterBatchSize         int              `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	ProductNegativeCacheTTLSeconds          int64            `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
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

func (c Config) ProductExistenceConfig() existencefilter.Config {
	expectedItems := c.ProductExistenceFilterExpectedItems
	if expectedItems == 0 {
		expectedItems = 1_000_000
	}
	falsePositiveRate := c.ProductExistenceFilterFalsePositiveRate
	if falsePositiveRate <= 0 || falsePositiveRate >= 1 {
		falsePositiveRate = 0.01
	}
	batchSize := c.ProductExistenceFilterBatchSize
	if batchSize <= 0 {
		batchSize = 1000
	}
	negativeTTL := time.Duration(c.ProductNegativeCacheTTLSeconds) * time.Second
	if negativeTTL <= 0 {
		negativeTTL = 30 * time.Second
	}
	prefix := strings.Trim(strings.TrimSpace(c.ProductExistenceFilterPrefix), ":")
	if prefix == "" {
		prefix = "flashmall:hertz:existence:product"
	}
	return existencefilter.Config{
		Prefix: prefix, NegativePrefix: "flashmall:hertz:negative",
		ExpectedItems: expectedItems, FalsePositiveRate: falsePositiveRate,
		BatchSize: batchSize, NegativeTTL: negativeTTL,
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
