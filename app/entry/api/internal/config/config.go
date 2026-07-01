package config

import (
	commonobs "flash-mall/app/common/observability"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf

	ProductRpcConf         zrpc.RpcClientConf
	OrderRpcConf           zrpc.RpcClientConf
	InventoryKitexEndpoint string
	RedisConf              redis.RedisConf
	Observability          commonobs.Config `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.

	DtmServer           string
	ProductRpcTarget    string
	OrderTimeoutSeconds int64
	OrderRpcTarget      string

	RequestIdTTLSeconds       int64
	DtmTimeoutToFailSeconds   int64
	DtmRequestTimeoutSeconds  int64
	DtmWaitResult             bool
	InventoryOwnsFinalDeduct  bool
	OrderRateLimitQps         int
	OrderRateLimitBurst       int
	OrderIdNode               int64
	StockShardCount           int `json:",optional"` //nolint:staticcheck // go-zero config uses optional in tags.
	DataSource                string
	CacheConf                 cache.CacheConf
	PprofAddr                 string
	MetricsAddr               string
	RabbitMQURL               string
	RabbitMQExchange          string
	RabbitMQQueue             string
	RabbitMQRouteKey          string
	RabbitMQConsumerTag       string
	RabbitMQPrefetch          int
	OrderEventConsumerEnabled bool
	EventDedupTTLSeconds      int64

	JwtAuthSecret                 string
	JwtExpireSeconds              int64
	AuthServiceBaseURL            string
	PaymentCallbackSecret         string
	PaymentCallbackMaxSkewSeconds int64

	CatalogProductIDs []int64 `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	UploadDir         string  `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
}
