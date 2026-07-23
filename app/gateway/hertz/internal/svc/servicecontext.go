package svc

import (
	"context"
	"strings"
	"time"

	"flash-mall/app/common/mysqlguard"
	"flash-mall/app/gateway/hertz/internal/adapters/authmysql"
	"flash-mall/app/gateway/hertz/internal/adapters/inventorykitex"
	"flash-mall/app/gateway/hertz/internal/adapters/ordermysql"
	"flash-mall/app/gateway/hertz/internal/adapters/orderrpc"
	"flash-mall/app/gateway/hertz/internal/adapters/productmysql"
	"flash-mall/app/gateway/hertz/internal/application/adminops"
	"flash-mall/app/gateway/hertz/internal/application/campaign"
	"flash-mall/app/gateway/hertz/internal/application/catalogquery"
	"flash-mall/app/gateway/hertz/internal/application/merchantonboarding"
	"flash-mall/app/gateway/hertz/internal/application/merchantquery"
	"flash-mall/app/gateway/hertz/internal/application/merchantstore"
	"flash-mall/app/gateway/hertz/internal/application/orderquery"
	"flash-mall/app/gateway/hertz/internal/application/productcommand"
	"flash-mall/app/gateway/hertz/internal/application/promotion"
	"flash-mall/app/gateway/hertz/internal/application/reconciliation"
	"flash-mall/app/gateway/hertz/internal/application/showcase"
	"flash-mall/app/gateway/hertz/internal/application/stockaudit"
	"flash-mall/app/gateway/hertz/internal/application/supplier"
	"flash-mall/app/gateway/hertz/internal/application/useraddress"
	"flash-mall/app/gateway/hertz/internal/assetstore"
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
	Config             config.Config
	SqlConn            sqlx.SqlConn
	OrderSqlConn       sqlx.SqlConn
	AuthSqlConn        sqlx.SqlConn
	OrderRpc           orderclient.Order
	ProductRpc         productclient.Product
	InventoryRpc       ports.InventoryService
	OrderCommands      ports.OrderCommands
	AdminOps           *adminops.Service
	Campaigns          *campaign.Service
	Reconciliation     *reconciliation.Service
	OrderQueries       *orderquery.Service
	BackofficeOrders   *orderquery.BackofficeService
	CatalogQueries     *catalogquery.Service
	MerchantQueries    *merchantquery.Service
	MerchantOnboarding *merchantonboarding.Service
	MerchantStores     *merchantstore.Service
	UserAddresses      *useraddress.Service
	ProductInventory   ports.ProductInventoryInitializer
	ProductCommands    *productcommand.Service
	ProductSnapshots   ports.ProductSnapshotStore
	Promotions         *promotion.Service
	Showcases          *showcase.Service
	StockAudits        *stockaudit.Service
	Suppliers          *supplier.Service
	Cache              *gatewaycache.Coordinator
	UploadIntegrity    *assetstore.IntegrityChecker
	cacheRedis         *redis.Client
}

func NewServiceContext(c config.Config) *ServiceContext {
	mysqlguard.MustUTF8MB4("hertz product", c.DataSource)
	mysqlguard.MustUTF8MB4("hertz order", c.OrderDataSource)
	mysqlguard.MustUTF8MB4("hertz auth", c.AuthDataSource)
	svcCtx := &ServiceContext{
		Config:       c,
		SqlConn:      sqlx.NewMysql(c.DataSource),
		OrderSqlConn: sqlx.NewMysql(c.OrderDataSource),
		AuthSqlConn:  sqlx.NewMysql(c.AuthDataSource),
		OrderRpc:     orderclient.NewOrder(zrpc.MustNewClient(c.OrderRpcConf)),
		ProductRpc:   productclient.NewProduct(zrpc.MustNewClient(c.ProductRpcConf)),
	}
	svcCtx.OrderCommands = orderrpc.New(svcCtx.OrderRpc)
	if orderDB, err := svcCtx.OrderSqlConn.RawDB(); err != nil {
		logx.Errorf("hertz order query adapter init failed: %v", err)
	} else {
		repository := ordermysql.NewQueryRepository(orderDB)
		svcCtx.AdminOps = adminops.NewService(ordermysql.NewAdminOpsRepository(orderDB))
		svcCtx.Reconciliation = reconciliation.NewService(ordermysql.NewReconciliationRepository(orderDB))
		svcCtx.OrderQueries = orderquery.NewService(repository)
		svcCtx.BackofficeOrders = orderquery.NewBackofficeService(repository)
		svcCtx.MerchantQueries = merchantquery.NewService(ordermysql.NewMerchantProfileRepository(orderDB))
		svcCtx.MerchantOnboarding = merchantonboarding.NewService(ordermysql.NewMerchantOnboardingRepository(orderDB))
		svcCtx.MerchantStores = merchantstore.NewService(ordermysql.NewMerchantStoreRepository(orderDB))
	}
	if authDB, err := svcCtx.AuthSqlConn.RawDB(); err != nil {
		logx.Errorf("hertz auth adapters init failed: %v", err)
	} else {
		svcCtx.UserAddresses = useraddress.NewService(authmysql.NewUserAddressRepository(authDB))
	}
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
		svcCtx.Campaigns = campaign.NewService(productmysql.NewCampaignRepository(productDB))
		svcCtx.ProductCommands = productcommand.NewService(productmysql.NewProductCommandRepository(productDB))
		svcCtx.CatalogQueries = catalogquery.NewService(productmysql.NewCatalogRepository(productDB))
		svcCtx.ProductSnapshots = productmysql.NewSnapshotStore(productDB)
		svcCtx.Promotions = promotion.NewService(productmysql.NewPromotionRepository(productDB), time.Now)
		svcCtx.Showcases = showcase.NewService(productmysql.NewShowcaseRepository(productDB))
		svcCtx.StockAudits = stockaudit.NewService(productmysql.NewStockAuditRepository(productDB))
		svcCtx.Suppliers = supplier.NewService(productmysql.NewSupplierRepository(productDB))
	}
	if orderDB, orderErr := svcCtx.OrderSqlConn.RawDB(); orderErr == nil && err == nil {
		svcCtx.UploadIntegrity = assetstore.NewIntegrityChecker(c.UploadDir, productDB, orderDB)
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
