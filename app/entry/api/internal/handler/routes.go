// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package handler

import (
	"net/http"

	"flash-mall/app/entry/api/internal/middleware"
	"flash-mall/app/entry/api/internal/svc"

	"github.com/zeromicro/go-zero/rest"
)

func RegisterHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	server.AddRoutes(
		[]rest.Route{
			{
				Method:  http.MethodGet,
				Path:    "/",
				Handler: HomeUIHandler(),
			},
			{
				Method:  http.MethodGet,
				Path:    "/shop",
				Handler: ShopUIHandler(),
			},
			{
				Method:  http.MethodGet,
				Path:    "/debug",
				Handler: DebugUIHandler(),
			},
			{
				Method:  http.MethodGet,
				Path:    "/admin",
				Handler: AdminUIHandler(),
			},
			{
				Method:  http.MethodGet,
				Path:    "/admin/*any",
				Handler: AdminUIHandler(),
			},
			{
				Method:  http.MethodGet,
				Path:    "/monitor",
				Handler: MonitorUIHandler(),
			},
			{
				Method:  http.MethodGet,
				Path:    "/js/*any",
				Handler: StaticWebAssetHandler("/js/"),
			},
			{
				Method:  http.MethodGet,
				Path:    "/styles/*any",
				Handler: StaticWebAssetHandler("/styles/"),
			},
			{
				Method:  http.MethodGet,
				Path:    "/uploads/products/*any",
				Handler: ProductUploadStaticHandler(serverCtx),
			},
			{
				Method:  http.MethodGet,
				Path:    "/api/system/health",
				Handler: SystemHealthHandler(serverCtx),
			},
			{
				Method:  http.MethodGet,
				Path:    "/api/shop/catalog",
				Handler: CatalogHandler(serverCtx),
			},
			{
				Method:  http.MethodGet,
				Path:    "/api/order/status",
				Handler: OrderStatusPollHandler(serverCtx),
			},
			{
				Method:  http.MethodGet,
				Path:    "/metrics",
				Handler: MetricsHandler(),
			},
		},
	)

	server.AddRoutes(
		[]rest.Route{
			{
				Method:  http.MethodPost,
				Path:    "/api/auth/login",
				Handler: AuthProxyHandler(serverCtx, "/api/auth/login"),
			},
			{
				Method:  http.MethodPost,
				Path:    "/api/auth/login/code",
				Handler: AuthProxyHandler(serverCtx, "/api/auth/login/code"),
			},
			{
				Method:  http.MethodPost,
				Path:    "/api/auth/register",
				Handler: AuthProxyHandler(serverCtx, "/api/auth/register"),
			},
			{
				Method:  http.MethodPost,
				Path:    "/api/auth/refresh",
				Handler: AuthProxyHandler(serverCtx, "/api/auth/refresh"),
			},
			{
				Method:  http.MethodPost,
				Path:    "/api/auth/logout",
				Handler: AuthProxyHandler(serverCtx, "/api/auth/logout"),
			},
			{
				Method:  http.MethodPost,
				Path:    "/api/auth/logout-all",
				Handler: AuthProxyHandler(serverCtx, "/api/auth/logout-all"),
			},
			{
				Method:  http.MethodGet,
				Path:    "/api/auth/me",
				Handler: AuthProxyHandler(serverCtx, "/api/auth/me"),
			},
			{
				Method:  http.MethodGet,
				Path:    "/api/auth/security/events/recent",
				Handler: AuthProxyHandler(serverCtx, "/api/auth/security/events/recent"),
			},
			{
				Method:  http.MethodPost,
				Path:    "/api/auth/code/send",
				Handler: AuthProxyHandler(serverCtx, "/api/auth/code/send"),
			},
			{
				Method:  http.MethodPost,
				Path:    "/api/auth/password/forgot",
				Handler: AuthProxyHandler(serverCtx, "/api/auth/password/forgot"),
			},
			{
				Method:  http.MethodPost,
				Path:    "/api/auth/password/reset",
				Handler: AuthProxyHandler(serverCtx, "/api/auth/password/reset"),
			},
			{
				Method:  http.MethodPost,
				Path:    "/api/payment/callback",
				Handler: PaymentCallbackHandler(serverCtx),
			},
		},
	)

	// Rate-limited order endpoints (JWT protected)
	server.AddRoutes(
		rest.WithMiddlewares(
			[]rest.Middleware{
				middleware.NewRateLimitMiddleware(serverCtx.OrderLimiter),
			},
			rest.Route{
				Method:  http.MethodPost,
				Path:    "/api/order/create",
				Handler: CreateOrderHandler(serverCtx),
			},
			rest.Route{
				Method:  http.MethodPost,
				Path:    "/api/order/pay",
				Handler: PayOrderHandler(serverCtx),
			},
			rest.Route{
				Method:  http.MethodGet,
				Path:    "/api/order/detail",
				Handler: OrderDetailHandler(serverCtx),
			},
		),
		rest.WithJwt(serverCtx.Config.JwtAuthSecret),
	)
	// JWT-protected order query endpoints (no rate limit needed)
	server.AddRoutes(
		[]rest.Route{
			{
				Method:  http.MethodGet,
				Path:    "/api/orders",
				Handler: OrderListHandler(serverCtx),
			},
			{
				Method:  http.MethodGet,
				Path:    "/api/orders/detail",
				Handler: OrderDetailHandler(serverCtx),
			},
		},
		rest.WithJwt(serverCtx.Config.JwtAuthSecret),
	)

	// JWT-protected order lifecycle endpoints
	server.AddRoutes(
		[]rest.Route{
			{
				Method:  http.MethodPost,
				Path:    "/api/order/cancel",
				Handler: CancelOrderHandler(serverCtx),
			},
			{
				Method:  http.MethodPost,
				Path:    "/api/order/confirm-receipt",
				Handler: ConfirmReceiptHandler(serverCtx),
			},
			{
				Method:  http.MethodPost,
				Path:    "/api/order/refund",
				Handler: RefundOrderHandler(serverCtx),
			},
			{
				Method:  http.MethodGet,
				Path:    "/api/user/addresses",
				Handler: UserAddressListHandler(serverCtx),
			},
			{
				Method:  http.MethodPost,
				Path:    "/api/user/addresses/upsert",
				Handler: UserAddressUpsertHandler(serverCtx),
			},
		},
		rest.WithJwt(serverCtx.Config.JwtAuthSecret),
	)

	// Merchant API routes (JWT protected, merchant membership checked per request)
	server.AddRoutes(
		[]rest.Route{
			{Method: http.MethodGet, Path: "/api/merchant/me", Handler: MerchantMeHandler(serverCtx)},
			{Method: http.MethodPost, Path: "/api/merchant/apply", Handler: MerchantApplyCreateHandler(serverCtx)},
			{Method: http.MethodGet, Path: "/api/merchant/dashboard/stats", Handler: MerchantDashboardStatsHandler(serverCtx)},
			{Method: http.MethodGet, Path: "/api/merchant/products", Handler: MerchantProductListHandler(serverCtx)},
			{Method: http.MethodPost, Path: "/api/merchant/products/create", Handler: MerchantProductCreateHandler(serverCtx)},
			{Method: http.MethodGet, Path: "/api/merchant/orders", Handler: MerchantOrderListHandler(serverCtx)},
			{Method: http.MethodPost, Path: "/api/merchant/orders/ship", Handler: MerchantShipOrderHandler(serverCtx)},
			{Method: http.MethodGet, Path: "/api/merchant/refunds", Handler: MerchantRefundListHandler(serverCtx)},
		},
		rest.WithJwt(serverCtx.Config.JwtAuthSecret),
	)

	// Admin API routes (JWT + admin role required)
	adminMW := []rest.Middleware{middleware.NewAdminAuthMiddleware()}
	server.AddRoutes(
		rest.WithMiddlewares(adminMW,
			rest.Route{Method: http.MethodGet, Path: "/api/admin/merchants/applications", Handler: AdminMerchantApplyListHandler(serverCtx)},
			rest.Route{Method: http.MethodPost, Path: "/api/admin/merchants/applications/audit", Handler: AdminMerchantApplyAuditHandler(serverCtx)},
			rest.Route{Method: http.MethodGet, Path: "/api/admin/orders", Handler: AdminOrderListHandler(serverCtx)},
			rest.Route{Method: http.MethodGet, Path: "/api/admin/orders/detail", Handler: AdminOrderDetailHandler(serverCtx)},
			rest.Route{Method: http.MethodGet, Path: "/api/admin/orders/status-logs", Handler: AdminOrderStatusLogHandler(serverCtx)},
			rest.Route{Method: http.MethodPost, Path: "/api/admin/orders/ship", Handler: AdminShipOrderHandler(serverCtx)},
			rest.Route{Method: http.MethodPost, Path: "/api/admin/orders/close", Handler: AdminCloseOrderHandler(serverCtx)},
			rest.Route{Method: http.MethodPost, Path: "/api/admin/orders/refund", Handler: AdminRefundOrderHandler(serverCtx)},
			rest.Route{Method: http.MethodGet, Path: "/api/admin/refunds", Handler: AdminRefundListHandler(serverCtx)},
			rest.Route{Method: http.MethodPost, Path: "/api/admin/refunds/audit", Handler: AdminRefundAuditHandler(serverCtx)},
			rest.Route{Method: http.MethodGet, Path: "/api/admin/reconciliation/issues", Handler: AdminReconciliationListHandler(serverCtx)},
			rest.Route{Method: http.MethodPost, Path: "/api/admin/reconciliation/scan", Handler: AdminReconciliationScanHandler(serverCtx)},
			rest.Route{Method: http.MethodGet, Path: "/api/admin/events", Handler: AdminEventListHandler(serverCtx)},
			rest.Route{Method: http.MethodPost, Path: "/api/admin/events/retry", Handler: AdminEventRetryHandler(serverCtx)},
			rest.Route{Method: http.MethodGet, Path: "/api/admin/products", Handler: AdminProductListHandler(serverCtx)},
			rest.Route{Method: http.MethodGet, Path: "/api/admin/products/detail", Handler: AdminProductDetailHandler(serverCtx)},
			rest.Route{Method: http.MethodPost, Path: "/api/admin/products/create", Handler: AdminProductCreateHandler(serverCtx)},
			rest.Route{Method: http.MethodPost, Path: "/api/admin/products/update", Handler: AdminProductUpdateHandler(serverCtx)},
			rest.Route{Method: http.MethodPost, Path: "/api/admin/products/stock-adjust", Handler: AdminProductStockAdjustHandler(serverCtx)},
			rest.Route{Method: http.MethodPost, Path: "/api/admin/products/image", Handler: AdminProductImageUploadHandler(serverCtx)},
			rest.Route{Method: http.MethodGet, Path: "/api/admin/suppliers", Handler: AdminSupplierListHandler(serverCtx)},
			rest.Route{Method: http.MethodGet, Path: "/api/admin/suppliers/detail", Handler: AdminSupplierDetailHandler(serverCtx)},
			rest.Route{Method: http.MethodPost, Path: "/api/admin/suppliers/create", Handler: AdminSupplierCreateHandler(serverCtx)},
			rest.Route{Method: http.MethodPost, Path: "/api/admin/suppliers/update", Handler: AdminSupplierUpdateHandler(serverCtx)},
			rest.Route{Method: http.MethodGet, Path: "/api/admin/promotions", Handler: AdminPromotionListHandler(serverCtx)},
			rest.Route{Method: http.MethodGet, Path: "/api/admin/promotions/detail", Handler: AdminPromotionDetailHandler(serverCtx)},
			rest.Route{Method: http.MethodPost, Path: "/api/admin/promotions/create", Handler: AdminPromotionCreateHandler(serverCtx)},
			rest.Route{Method: http.MethodPost, Path: "/api/admin/promotions/update", Handler: AdminPromotionUpdateHandler(serverCtx)},
			rest.Route{Method: http.MethodGet, Path: "/api/admin/campaigns", Handler: AdminCampaignListHandler(serverCtx)},
			rest.Route{Method: http.MethodPost, Path: "/api/admin/campaigns/upsert", Handler: AdminCampaignUpsertHandler(serverCtx)},
			rest.Route{Method: http.MethodGet, Path: "/api/admin/users", Handler: AdminUserListHandler(serverCtx)},
			rest.Route{Method: http.MethodGet, Path: "/api/admin/users/detail", Handler: AdminUserDetailHandler(serverCtx)},
			rest.Route{Method: http.MethodPost, Path: "/api/admin/users/status", Handler: AuthProxyHandler(serverCtx, "/api/admin/users/status")},
			rest.Route{Method: http.MethodGet, Path: "/api/admin/security/events/recent", Handler: AuthProxyHandler(serverCtx, "/api/admin/security/events/recent")},
			rest.Route{Method: http.MethodGet, Path: "/api/admin/dashboard/stats", Handler: AdminDashboardStatsHandler(serverCtx)},
		),
		rest.WithJwt(serverCtx.Config.JwtAuthSecret),
	)
}
