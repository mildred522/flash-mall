package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf

	ProductRpcConf zrpc.RpcClientConf
	OrderRpcConf   zrpc.RpcClientConf
	RedisConf      redis.RedisConf

	DtmServer           string
	ProductRpcTarget    string
	OrderTimeoutSeconds int64
	OrderRpcTarget      string

	RequestIdTTLSeconds       int64
	DtmTimeoutToFailSeconds   int64
	DtmRequestTimeoutSeconds  int64
	DtmWaitResult             bool
	OrderRateLimitQps         int
	OrderRateLimitBurst       int
	OrderIdNode               int64
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

	JwtAuthSecret    string
	JwtExpireSeconds int64
	AuthDemoPassword string
}
