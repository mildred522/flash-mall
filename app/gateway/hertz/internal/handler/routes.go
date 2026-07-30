package handler

import (
	"time"

	"flash-mall/app/gateway/hertz/internal/middleware"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app/server"
)

func RegisterRoutes(h *server.Hertz, svcCtx *svc.ServiceContext, startedAt time.Time) {
	h.Use(middleware.Recover(), middleware.Trace(), middleware.ObserveRequests(), middleware.AccessLog())

	registerSystemRoutes(h, svcCtx, startedAt)
	registerAuthRoutes(h, svcCtx)
	registerShopRoutes(h, svcCtx)
	registerInventoryRoutes(h, svcCtx)
	registerOrderRoutes(h, svcCtx)
	registerAdminRoutes(h, svcCtx)
	registerMerchantRoutes(h, svcCtx)
}
