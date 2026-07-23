package handler

import (
	"context"
	"fmt"
	"time"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/ports"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

const inventoryReadinessTimeout = 750 * time.Millisecond

type runtimeStateClient interface {
	GetRuntimeState(context.Context, ports.RequestMeta) (ports.InventoryRuntimeState, error)
}

func HealthHandler(svcCtx *svc.ServiceContext, startedAt time.Time) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if svcCtx.Config.InventoryKitexEndpoint != "" && svcCtx.InventoryRpc == nil {
			fail(ctx, c, consts.StatusServiceUnavailable, apperror.New(apperror.CodeInternal, "inventory kitex client unavailable"))
			return
		}
		var inventoryRuntime any
		if svcCtx.InventoryRpc != nil {
			state, err := checkInventoryRuntime(ctx, svcCtx.InventoryRpc)
			if err != nil {
				fail(ctx, c, consts.StatusServiceUnavailable, err)
				return
			}
			inventoryRuntime = state
		}
		var uploadStorage any
		if svcCtx.UploadIntegrity != nil {
			report, err := svcCtx.UploadIntegrity.Check(ctx)
			if err != nil {
				fail(ctx, c, consts.StatusServiceUnavailable, apperror.Wrap(apperror.CodeInternal, "upload storage check failed", err))
				return
			}
			if !report.Healthy() {
				fail(ctx, c, consts.StatusServiceUnavailable, apperror.New(apperror.CodeInternal,
					fmt.Sprintf("upload storage is not healthy: writable=%t missing_files=%d", report.Writable, report.MissingFiles)))
				return
			}
			uploadStorage = report
		}
		ok(ctx, c, map[string]any{
			"name":                       svcCtx.Config.Name,
			"status":                     "ok",
			"service":                    "hertz-gateway",
			"uptime_ms":                  time.Since(startedAt).Milliseconds(),
			"server_time":                time.Now().Unix(),
			"inventory_kitex_configured": svcCtx.Config.InventoryKitexEndpoint != "",
			"inventory_runtime":          inventoryRuntime,
			"live_stock_overlay_enabled": svcCtx.Config.EnableLiveStockOverlay,
			"upload_storage":             uploadStorage,
		})
	}
}

func checkInventoryRuntime(ctx context.Context, client runtimeStateClient) (ports.InventoryRuntimeState, error) {
	checkCtx, cancel := context.WithTimeout(ctx, inventoryReadinessTimeout)
	defer cancel()
	state, err := client.GetRuntimeState(checkCtx, inventoryRequestMeta(ctx))
	if err != nil {
		return ports.InventoryRuntimeState{}, apperror.Wrap(apperror.CodeInternal, "inventory runtime state check failed", err)
	}
	if !state.FinalDeductEnabled || !state.RedisConfigured || !state.MySQLConfigured || state.ShardCount <= 0 || !state.ReservationLedgerEnabled {
		return state, apperror.New(apperror.CodeInternal, fmt.Sprintf(
			"inventory runtime is not authoritative: final_deduct=%t redis=%t mysql=%t shard_count=%d ledger=%s",
			state.FinalDeductEnabled, state.RedisConfigured, state.MySQLConfigured, state.ShardCount, state.ReservationLedgerMode,
		))
	}
	return state, nil
}
