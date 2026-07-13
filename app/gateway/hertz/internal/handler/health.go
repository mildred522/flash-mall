package handler

import (
	"context"
	"time"

	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
)

func HealthHandler(svcCtx *svc.ServiceContext, startedAt time.Time) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		ok(ctx, c, map[string]any{
			"name":                       svcCtx.Config.Name,
			"status":                     "ok",
			"service":                    "hertz-gateway",
			"uptime_ms":                  time.Since(startedAt).Milliseconds(),
			"server_time":                time.Now().Unix(),
			"inventory_kitex_configured": svcCtx.Config.InventoryKitexEndpoint != "",
			"live_stock_overlay_enabled": svcCtx.Config.EnableLiveStockOverlay,
		})
	}
}
