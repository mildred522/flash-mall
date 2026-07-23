package svc

import (
	"errors"
	"strings"

	"flash-mall/app/common/mysqlguard"
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
	validateInventoryReserveConfig(c)
	mysqlguard.MustUTF8MB4("order", c.DataSource)
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
			if c.RequireInventoryReserve {
				logx.Must(err)
			}
		} else {
			svcCtx.InventoryClient = client
		}
	}
	if c.RequireInventoryReserve && svcCtx.InventoryClient == nil {
		logx.Must(errors.New("RequireInventoryReserve=true requires a ready inventory kitex client"))
	}
	logInventoryReserveMode(c, svcCtx.InventoryClient != nil)

	return svcCtx
}

func validateInventoryReserveConfig(c config.Config) {
	if !c.RequireInventoryReserve {
		return
	}
	if strings.TrimSpace(c.InventoryKitexEndpoint) == "" {
		logx.Must(errors.New("RequireInventoryReserve=true requires InventoryKitexEndpoint"))
	}
}

func logInventoryReserveMode(c config.Config, inventoryClientReady bool) {
	endpoint := strings.TrimSpace(c.InventoryKitexEndpoint)
	reserveWriter := "order-rpc-redis"
	if endpoint != "" && inventoryClientReady {
		reserveWriter = "inventory-kitex"
	}
	logx.Infof(
		"order inventory reserve mode: reserve_writer=%s inventory_endpoint=%q require_inventory_reserve=%t inventory_client_ready=%t",
		reserveWriter,
		endpoint,
		c.RequireInventoryReserve,
		inventoryClientReady,
	)
}
