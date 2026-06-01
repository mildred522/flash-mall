package handler

import (
	"net/http"

	"flash-mall/app/order/api/internal/svc"
	"flash-mall/app/order/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func AdminDashboardStatsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		var stats types.AdminDashboardStats

		_ = db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM orders").Scan(&stats.TotalOrders)
		_ = db.QueryRowContext(r.Context(), "SELECT COALESCE(SUM(payable_amount_fen), 0) FROM orders WHERE status IN (1,3,4)").Scan(&stats.TotalRevenueFen)
		_ = db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM product").Scan(&stats.TotalProducts)
		_ = db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM orders WHERE status = 0").Scan(&stats.PendingOrders)
		_ = db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM orders WHERE status = 1").Scan(&stats.PaidOrders)
		_ = db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM orders WHERE status = 3").Scan(&stats.ShippedOrders)
		_ = db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM orders WHERE status = 4").Scan(&stats.CompletedOrders)

		// User count from auth-api
		stats.TotalUsers = 0 // Will be populated if auth-api is available

		logx.WithContext(r.Context()).Infof("dashboard stats: orders=%d revenue=%d products=%d", stats.TotalOrders, stats.TotalRevenueFen, stats.TotalProducts)

		httpx.OkJsonCtx(r.Context(), w, stats)
	}
}
