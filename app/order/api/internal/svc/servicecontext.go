package svc

import (
	"flash-mall/app/order/api/internal/config"
	"flash-mall/app/order/api/internal/idgen"
	"flash-mall/app/order/api/internal/model"
	orderClient "flash-mall/app/order/rpc/orderclient"
	productClient "flash-mall/app/product/rpc/productclient"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
	"golang.org/x/time/rate"
)

type ServiceContext struct {
	Config     config.Config
	OrderRpc   orderClient.Order
	ProductRpc productClient.Product
	Redis      *redis.Redis
	SqlConn    sqlx.SqlConn
	OrderModel model.OrdersModel
	// CHG 2026-02-24: 变更=新增下单限流器; 之前=无显式限流; 原因=高峰期快速失败保护后端。
	OrderLimiter *rate.Limiter
	OrderIdGen   idgen.Generator
}

func NewServiceContext(c config.Config) *ServiceContext {
	sqlConn := sqlx.NewMysql(c.DataSource)
	orderIDGen, err := idgen.NewSnowflakeGenerator(c.OrderIdNode)
	logx.Must(err)
	logx.Infof("order id generator initialized, node_id=%d", orderIDGen.NodeID())

	var limiter *rate.Limiter
	if c.OrderRateLimitQps > 0 {
		burst := c.OrderRateLimitBurst
		if burst <= 0 {
			burst = c.OrderRateLimitQps * 2
		}
		limiter = rate.NewLimiter(rate.Limit(c.OrderRateLimitQps), burst)
	}
	return &ServiceContext{
		Config:       c,
		OrderRpc:     orderClient.NewOrder(zrpc.MustNewClient(c.OrderRpcConf)),
		ProductRpc:   productClient.NewProduct(zrpc.MustNewClient(c.ProductRpcConf)),
		Redis:        redis.MustNewRedis(c.RedisConf),
		SqlConn:      sqlConn,
		OrderModel:   model.NewOrdersModel(sqlConn, c.CacheConf),
		OrderLimiter: limiter,
		OrderIdGen:   orderIDGen,
	}
}
