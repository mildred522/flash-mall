package svc

import (
	"flash-mall/app/order/rpc/internal/config"
	productclient "flash-mall/app/product/rpc/productclient"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config     config.Config
	SqlConn    sqlx.SqlConn
	Redis      *redis.Redis
	ProductRpc productclient.Product
}

func NewServiceContext(c config.Config) *ServiceContext {
	svcCtx := &ServiceContext{
		Config:  c,
		SqlConn: sqlx.NewMysql(c.DataSource),
		Redis:   redis.MustNewRedis(c.RedisConf),
	}

	if target, err := c.ProductRpcConf.BuildTarget(); err == nil && target != "" {
		svcCtx.ProductRpc = productclient.NewProduct(zrpc.MustNewClient(c.ProductRpcConf))
	}

	return svcCtx
}
