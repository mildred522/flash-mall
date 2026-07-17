package svc

import (
	"context"
	"strings"
	"time"

	"flash-mall/app/gateway/hertz/internal/adapters/inventorykitex"
	"flash-mall/app/gateway/hertz/internal/adapters/orderrpc"
	"flash-mall/app/gateway/hertz/internal/adapters/productmysql"
	gatewaycache "flash-mall/app/gateway/hertz/internal/cache"
	"flash-mall/app/gateway/hertz/internal/config"
	"flash-mall/app/gateway/hertz/internal/ports"
	orderclient "flash-mall/app/order/rpc/orderclient"
	productclient "flash-mall/app/product/rpc/productclient"

	redis "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config           config.Config
	SqlConn          sqlx.SqlConn
	OrderSqlConn     sqlx.SqlConn
	AuthSqlConn      sqlx.SqlConn
	OrderRpc         orderclient.Order
	ProductRpc       productclient.Product
	InventoryRpc     ports.InventoryService
	OrderCommands    ports.OrderCommands
	ProductInventory ports.ProductInventoryInitializer
	ProductSnapshots ports.ProductSnapshotStore
	Cache            *gatewaycache.Coordinator
	cacheRedis       *redis.Client
}

func NewServiceContext(c config.Config) *ServiceContext {
	svcCtx := &ServiceContext{
		Config:       c,
		SqlConn:      sqlx.NewMysql(c.DataSource),
		OrderSqlConn: sqlx.NewMysql(c.OrderDataSource),
		AuthSqlConn:  sqlx.NewMysql(c.AuthDataSource),
		OrderRpc:     orderclient.NewOrder(zrpc.MustNewClient(c.OrderRpcConf)),
		ProductRpc:   productclient.NewProduct(zrpc.MustNewClient(c.ProductRpcConf)),
	}
	svcCtx.OrderCommands = orderrpc.New(svcCtx.OrderRpc)
	if c.InventoryKitexEndpoint != "" {
		client, err := inventorykitex.New(c.InventoryKitexEndpoint)
		if err != nil {
			logx.Errorf("hertz inventory kitex client init failed: endpoint=%s err=%v", c.InventoryKitexEndpoint, err)
		} else {
			svcCtx.InventoryRpc = client
		}
	}
	productDB, err := svcCtx.SqlConn.RawDB()
	if err != nil {
		logx.Errorf("hertz product adapters init failed: %v", err)
	} else {
		svcCtx.ProductInventory = productmysql.NewInventoryInitializer(productDB, svcCtx.InventoryRpc)
		svcCtx.ProductSnapshots = productmysql.NewSnapshotStore(productDB)
	}
	cacheConfig := c.CacheConfig()
	var cacheRedis *redis.Client
	if cacheConfig.EnableL2 && strings.TrimSpace(c.CacheRedisAddr) != "" {
		cacheRedis = redis.NewClient(&redis.Options{
			Addr:         strings.TrimSpace(c.CacheRedisAddr),
			MaxRetries:   -1,
			DialTimeout:  200 * time.Millisecond,
			ReadTimeout:  500 * time.Millisecond,
			WriteTimeout: 500 * time.Millisecond,
		})
		svcCtx.cacheRedis = cacheRedis
	}
	svcCtx.Cache = gatewaycache.New(cacheConfig, cacheRedis)
	svcCtx.Cache.Start(context.Background())
	return svcCtx
}

func (s *ServiceContext) Close() {
	if s.Cache != nil {
		_ = s.Cache.Close()
	}
	if s.cacheRedis != nil {
		_ = s.cacheRedis.Close()
	}
}
