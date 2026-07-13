package svc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"flash-mall/app/entry/api/internal/cache"
	"flash-mall/app/entry/api/internal/config"
	"flash-mall/app/entry/api/internal/idgen"
	"flash-mall/app/entry/api/internal/inventoryclient"
	"flash-mall/app/entry/api/internal/model"
	"flash-mall/app/entry/api/internal/sessionstate"
	orderClient "flash-mall/app/order/rpc/orderclient"
	productClient "flash-mall/app/product/rpc/productclient"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
	"golang.org/x/time/rate"
)

type ServiceContext struct {
	Config           config.Config
	OrderRpc         orderClient.Order
	ProductRpc       productClient.Product
	InventoryClient  inventoryclient.Client
	Redis            *redis.Redis
	SqlConn          sqlx.SqlConn
	OrderModel       model.OrdersModel
	SessionValidator sessionstate.Validator
	// CHG 2026-02-24: 变更=新增下单限流器; 之前=无显式限流; 原因=高峰期快速失败保护后端。
	OrderLimiter *rate.Limiter
	OrderIdGen   idgen.Generator
	CatalogCache *cache.CatalogCache
	StartTime    time.Time
}

func NewServiceContext(c config.Config) *ServiceContext {
	validateInventoryMigrationConfig(c)
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
	rds := redis.MustNewRedis(c.RedisConf)
	go repairRuntimeStockShards(context.Background(), sqlConn, rds, c.StockShardCount)
	var validator sessionstate.Validator
	validator = sessionstate.NewHTTPValidator(nil, c.AuthServiceBaseURL)
	if c.JwtAuthSecret != "" {
		validator = sessionstate.NewRedisValidator(rds, c.JwtAuthSecret)
	}
	var inventoryClient inventoryclient.Client
	if c.InventoryKitexEndpoint != "" {
		client, err := inventoryclient.NewKitexClient(c.InventoryKitexEndpoint)
		if err != nil {
			logx.Errorf("inventory kitex client disabled: endpoint=%s err=%v", c.InventoryKitexEndpoint, err)
			if c.InventoryOwnsFinalDeduct {
				logx.Must(err)
			}
		} else {
			inventoryClient = client
		}
	}
	if c.InventoryOwnsFinalDeduct && inventoryClient == nil {
		logx.Must(errors.New("InventoryOwnsFinalDeduct=true requires a ready inventory kitex client"))
	}
	return &ServiceContext{
		Config:           c,
		OrderRpc:         orderClient.NewOrder(zrpc.MustNewClient(c.OrderRpcConf)),
		ProductRpc:       productClient.NewProduct(zrpc.MustNewClient(c.ProductRpcConf)),
		InventoryClient:  inventoryClient,
		Redis:            rds,
		SqlConn:          sqlConn,
		OrderModel:       model.NewOrdersModel(sqlConn, c.CacheConf),
		SessionValidator: validator,
		OrderLimiter:     limiter,
		OrderIdGen:       orderIDGen,
		CatalogCache:     cache.NewCatalogCache(rds),
		StartTime:        time.Now(),
	}
}

func validateInventoryMigrationConfig(c config.Config) {
	if !c.InventoryOwnsFinalDeduct {
		return
	}
	if strings.TrimSpace(c.InventoryKitexEndpoint) == "" {
		logx.Must(errors.New("InventoryOwnsFinalDeduct=true requires InventoryKitexEndpoint"))
	}
}

func repairRuntimeStockShards(ctx context.Context, sqlConn sqlx.SqlConn, rds *redis.Redis, shardCount int) {
	if rds == nil {
		return
	}
	db, err := sqlConn.RawDB()
	if err != nil {
		logx.Errorf("repair runtime stock shards skipped: raw db failed: %v", err)
		return
	}
	rows, err := db.QueryContext(ctx, `
SELECT p.id, COALESCE(bucket.stock_total, p.stock, 0) AS stock_total
FROM mall_product.product p
LEFT JOIN (
  SELECT product_id, COALESCE(SUM(stock), 0) AS stock_total
  FROM mall_product.product_stock_bucket
  GROUP BY product_id
) bucket ON bucket.product_id = p.id
WHERE p.status = 1`)
	if err != nil {
		logx.Errorf("repair runtime stock shards query failed: %v", err)
		return
	}
	defer func() { _ = rows.Close() }()

	repaired := 0
	for rows.Next() {
		var productID int64
		var total sql.NullInt64
		if err := rows.Scan(&productID, &total); err != nil {
			logx.Errorf("repair runtime stock shards scan failed: %v", err)
			return
		}
		stockTotal := int64(0)
		if total.Valid {
			stockTotal = total.Int64
		}
		if err := writeRuntimeStockShards(ctx, rds, productID, stockTotal, shardCount); err != nil {
			logx.Errorf("repair runtime stock shards failed: product_id=%d err=%v", productID, err)
			continue
		}
		repaired++
	}
	if err := rows.Err(); err != nil {
		logx.Errorf("repair runtime stock shards rows failed: %v", err)
		return
	}
	if repaired > 0 {
		logx.Infof("repair runtime stock shards completed: products=%d", repaired)
	}
}

func writeRuntimeStockShards(ctx context.Context, rds *redis.Redis, productID int64, total int64, shardCount int) error {
	if total < 0 {
		total = 0
	}
	if shardCount <= 0 {
		shardCount = 1
	}
	perShard := total / int64(shardCount)
	remain := total % int64(shardCount)
	for shardIdx := 0; shardIdx < shardCount; shardIdx++ {
		value := perShard
		if shardIdx == 0 {
			value += remain
		}
		if err := rds.SetCtx(ctx, fmt.Sprintf("stock:%d:%d", productID, shardIdx), fmt.Sprintf("%d", value)); err != nil {
			return err
		}
	}
	return nil
}
