package svc

import (
	"flash-mall/app/order/rpc/internal/config"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config  config.Config
	SqlConn sqlx.SqlConn
	Redis   *redis.Redis
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config:  c,
		SqlConn: sqlx.NewMysql(c.DataSource),
		Redis:   redis.MustNewRedis(c.RedisConf),
	}
}
