package svc

import (
	"flash-mall/app/gateway/hertz/internal/config"
	"flash-mall/app/gateway/hertz/internal/inventoryclient"
	orderclient "flash-mall/app/order/rpc/orderclient"
	productclient "flash-mall/app/product/rpc/productclient"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config       config.Config
	SqlConn      sqlx.SqlConn
	OrderSqlConn sqlx.SqlConn
	AuthSqlConn  sqlx.SqlConn
	OrderRpc     orderclient.Order
	ProductRpc   productclient.Product
	InventoryRpc inventoryclient.Client
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
	if c.InventoryKitexEndpoint != "" {
		client, err := inventoryclient.NewKitexClient(c.InventoryKitexEndpoint)
		if err != nil {
			logx.Errorf("hertz inventory kitex client init failed: endpoint=%s err=%v", c.InventoryKitexEndpoint, err)
		} else {
			svcCtx.InventoryRpc = client
		}
	}
	return svcCtx
}
