package config

import (
	commonobs "flash-mall/app/common/observability"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	DataSource     string
	RedisConf      redis.RedisConf
	ProductRpcConf zrpc.RpcClientConf `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.
	Observability  commonobs.Config   `json:",optional"` //nolint:staticcheck // go-zero config uses optional in json tags.

	PprofAddr   string
	MetricsAddr string

	StockShardCount int

	// RabbitMQ outbox publisher configs.
	RabbitMQURL      string
	RabbitMQExchange string
	RabbitMQRouteKey string
	OutboxBatchSize  int
	OutboxPollMs     int
	OutboxRetrySec   int
	OutboxTimeoutSec int
	OutboxMaxRetries int

	// Outbox single-active publisher configs.
	OutboxSingleActive  bool
	OutboxLeaderLockKey string
	OutboxLeaderLockTTL int
}
