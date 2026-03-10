package config

import (
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	DataSource string
	RedisConf  redis.RedisConf

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
