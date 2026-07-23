package handler

import (
	"time"

	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app/server"
)

func registerSystemRoutes(h *server.Hertz, svcCtx *svc.ServiceContext, startedAt time.Time) {
	h.GET("/", StaticPageHandler("shop.html"))
	h.GET("/shop", StaticPageHandler("shop.html"))
	h.GET("/pay", StaticPageHandler("shop.html"))
	h.GET("/product/*any", StaticPageHandler("shop.html"))
	h.GET("/store/*any", StaticPageHandler("shop.html"))
	h.GET("/admin", StaticPageHandler("admin.html"))
	h.GET("/admin/*any", StaticPageHandler("admin.html"))
	h.GET("/merchant", StaticPageHandler("merchant.html"))
	h.GET("/merchant/*any", StaticPageHandler("merchant.html"))
	h.GET("/live", LivenessHandler(svcCtx, startedAt))
	h.GET("/ready", HealthHandler(svcCtx, startedAt))
	h.GET("/health", HealthHandler(svcCtx, startedAt))
	h.GET("/api/system/health", HealthHandler(svcCtx, startedAt))
	h.GET("/api/system/migration/routes", RouteMigrationStatusHandler())
	h.GET("/metrics", MetricsHandler())
}
