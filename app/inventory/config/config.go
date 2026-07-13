package config

import "github.com/zeromicro/go-zero/core/stores/redis"

type Config struct {
	Name               string
	ListenOn           string
	DataSource         string
	RedisConf          redis.RedisConf
	StockShardCount    int
	FinalDeductEnabled bool
}

func (c Config) NormalizedStockShardCount() int {
	if c.StockShardCount <= 0 {
		return 1
	}
	return c.StockShardCount
}
