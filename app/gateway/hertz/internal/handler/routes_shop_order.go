package handler

import (
	"flash-mall/app/gateway/hertz/internal/middleware"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app/server"
)

func registerShopRoutes(h *server.Hertz, svcCtx *svc.ServiceContext) {
	h.GET("/products/*any", StaticAssetHandler("products"))
	h.GET("/uploads/products/*any", ProductUploadStaticHandler(svcCtx))
	h.GET("/uploads/stores/*any", StoreUploadStaticHandler(svcCtx))
	h.GET("/api/shop/catalog", CatalogHandler(svcCtx))
	h.GET("/api/shop/products", ProductListHandler(svcCtx, true))
	h.GET("/api/shop/products/detail", ProductDetailHandler(svcCtx))
	h.GET("/api/shop/stores/detail", StoreDetailHandler(svcCtx))
	h.GET("/api/shop/stores/products", StoreProductListHandler(svcCtx))
	h.GET("/api/user/addresses", middleware.RequireUser(svcCtx.Config.JwtAuthSecret), UserAddressListHandler(svcCtx))
	h.POST("/api/user/addresses/upsert", middleware.RequireUser(svcCtx.Config.JwtAuthSecret), UserAddressUpsertHandler(svcCtx))
}

func registerAuthRoutes(h *server.Hertz, svcCtx *svc.ServiceContext) {
	h.POST("/api/auth/login", AuthProxyHandler(svcCtx, "/api/auth/login"))
	h.POST("/api/auth/login/code", AuthProxyHandler(svcCtx, "/api/auth/login/code"))
	h.POST("/api/auth/register", AuthProxyHandler(svcCtx, "/api/auth/register"))
	h.POST("/api/auth/refresh", AuthProxyHandler(svcCtx, "/api/auth/refresh"))
	h.POST("/api/auth/logout", AuthProxyHandler(svcCtx, "/api/auth/logout"))
	h.POST("/api/auth/logout-all", AuthProxyHandler(svcCtx, "/api/auth/logout-all"))
	h.GET("/api/auth/me", AuthProxyHandler(svcCtx, "/api/auth/me"))
	h.GET("/api/auth/security/events/recent", AuthProxyHandler(svcCtx, "/api/auth/security/events/recent"))
	h.POST("/api/auth/code/send", AuthProxyHandler(svcCtx, "/api/auth/code/send"))
	h.POST("/api/auth/password/forgot", AuthProxyHandler(svcCtx, "/api/auth/password/forgot"))
	h.POST("/api/auth/password/reset", AuthProxyHandler(svcCtx, "/api/auth/password/reset"))
	h.POST("/api/payment/callback", PaymentCallbackHandler(svcCtx))
	h.GET("/api/payment/status", middleware.OptionalIdentity(svcCtx.Config.JwtAuthSecret), PaymentStatusHandler(svcCtx))
	h.POST("/api/payment/sandbox/confirm", SandboxPaymentConfirmHandler(svcCtx))
}

func registerInventoryRoutes(h *server.Hertz, svcCtx *svc.ServiceContext) {
	h.GET("/api/inventory/summary", InventorySummaryHandler(svcCtx))
}

func registerOrderRoutes(h *server.Hertz, svcCtx *svc.ServiceContext) {
	h.GET("/api/order/status", middleware.RequireUser(svcCtx.Config.JwtAuthSecret), OrderStatusPollHandler(svcCtx))
	h.POST("/api/order/create", middleware.RequireUser(svcCtx.Config.JwtAuthSecret), CreateOrderHandler(svcCtx))
	h.POST("/api/order/pay", middleware.RequireUser(svcCtx.Config.JwtAuthSecret), PayOrderHandler(svcCtx))
	h.POST("/api/order/cancel", middleware.RequireUser(svcCtx.Config.JwtAuthSecret), CancelOrderHandler(svcCtx))
	h.POST("/api/order/refund", middleware.RequireUser(svcCtx.Config.JwtAuthSecret), RefundOrderHandler(svcCtx))
	h.POST("/api/order/confirm-receipt", middleware.RequireUser(svcCtx.Config.JwtAuthSecret), ConfirmReceiptHandler(svcCtx))
	h.GET("/api/order/detail", middleware.RequireUser(svcCtx.Config.JwtAuthSecret), OrderDetailHandler(svcCtx))
	h.GET("/api/orders", middleware.RequireUser(svcCtx.Config.JwtAuthSecret), OrderListHandler(svcCtx))
	h.GET("/api/orders/detail", middleware.RequireUser(svcCtx.Config.JwtAuthSecret), OrderDetailHandler(svcCtx))
}
