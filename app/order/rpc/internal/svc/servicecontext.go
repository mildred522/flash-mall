package svc

import (
	"strings"

	"flash-mall/app/order/rpc/internal/config"
	"flash-mall/app/order/rpc/internal/inventoryclient"
	productclient "flash-mall/app/product/rpc/productclient"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config          config.Config
	SqlConn         sqlx.SqlConn
	Redis           *redis.Redis
	ProductRpc      productclient.Product
	InventoryClient inventoryclient.Client
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
	if endpoint := strings.TrimSpace(c.InventoryKitexEndpoint); endpoint != "" {
		client, err := inventoryclient.NewKitexClient(endpoint)
		if err != nil {
			logx.Errorf("inventory kitex client init failed: endpoint=%s err=%v", endpoint, err)
		} else {
			svcCtx.InventoryClient = client
		}
	}

	return svcCtx
}
