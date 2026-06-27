package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"flash-mall/app/common/orderstatus"
	"flash-mall/app/entry/api/internal/svc"
	"flash-mall/app/entry/api/internal/types"

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

		stats, err := loadAdminDashboardStats(r.Context(), db)
		if err != nil {
			logx.WithContext(r.Context()).Errorf("dashboard stats query failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		stats.TotalUsers = fetchAdminUserTotal(r, svcCtx)

		logx.WithContext(r.Context()).Infof("dashboard stats: orders=%d revenue=%d products=%d", stats.TotalOrders, stats.TotalRevenueFen, stats.TotalProducts)

		httpx.OkJsonCtx(r.Context(), w, stats)
	}
}

func loadAdminDashboardStats(ctx context.Context, db *sql.DB) (types.AdminDashboardStats, error) {
	var stats types.AdminDashboardStats
	queries := []struct {
		name  string
		dest  *int64
		query string
		args  []any
	}{
		{name: "total_orders", dest: &stats.TotalOrders, query: "SELECT COUNT(*) FROM orders"},
		{name: "total_revenue", dest: &stats.TotalRevenueFen, query: `
SELECT COALESCE(SUM(s.payable_amount_fen), 0)
FROM orders o
JOIN order_price_snapshot s ON s.order_id = o.id
WHERE o.status IN (?,?,?)`, args: []any{orderstatus.Paid, orderstatus.Shipped, orderstatus.Completed}},
		{name: "total_products", dest: &stats.TotalProducts, query: "SELECT COUNT(*) FROM mall_product.product"},
		{name: "total_suppliers", dest: &stats.TotalSuppliers, query: "SELECT COUNT(*) FROM mall_product.supplier"},
		{name: "total_promotions", dest: &stats.TotalPromotions, query: "SELECT COUNT(*) FROM mall_product.promotion_rule"},
		{name: "active_promotions", dest: &stats.ActivePromotions, query: "SELECT COUNT(*) FROM mall_product.promotion_rule WHERE status = 1 AND (starts_at IS NULL OR starts_at <= NOW()) AND (ends_at IS NULL OR ends_at >= NOW())"},
		{name: "low_stock_products", dest: &stats.LowStockProducts, query: `
SELECT COUNT(*)
FROM (
  SELECT p.id, COALESCE(SUM(b.stock), 0) AS stock_available
  FROM mall_product.product p
  LEFT JOIN mall_product.product_stock_bucket b ON b.product_id = p.id
  WHERE p.status = 1
  GROUP BY p.id
  HAVING stock_available > 0 AND stock_available <= 100
) low_stock`},
		{name: "out_of_stock_products", dest: &stats.OutOfStockProducts, query: `
SELECT COUNT(*)
FROM (
  SELECT p.id, COALESCE(SUM(b.stock), 0) AS stock_available
  FROM mall_product.product p
  LEFT JOIN mall_product.product_stock_bucket b ON b.product_id = p.id
  WHERE p.status = 1
  GROUP BY p.id
  HAVING stock_available = 0
) out_of_stock`},
		{name: "pending_orders", dest: &stats.PendingOrders, query: "SELECT COUNT(*) FROM orders WHERE status = ?", args: []any{orderstatus.PendingPayment}},
		{name: "paid_orders", dest: &stats.PaidOrders, query: "SELECT COUNT(*) FROM orders WHERE status = ?", args: []any{orderstatus.Paid}},
		{name: "shipped_orders", dest: &stats.ShippedOrders, query: "SELECT COUNT(*) FROM orders WHERE status = ?", args: []any{orderstatus.Shipped}},
		{name: "completed_orders", dest: &stats.CompletedOrders, query: "SELECT COUNT(*) FROM orders WHERE status = ?", args: []any{orderstatus.Completed}},
		{name: "refund_requested", dest: &stats.RefundRequested, query: "SELECT COUNT(*) FROM orders WHERE status = ?", args: []any{orderstatus.RefundRequested}},
		{name: "refunded_orders", dest: &stats.RefundedOrders, query: "SELECT COUNT(*) FROM orders WHERE status = ?", args: []any{orderstatus.Refunded}},
		{name: "open_reconciliation_issues", dest: &stats.OpenReconIssues, query: "SELECT COUNT(*) FROM reconciliation_issue WHERE status = 0"},
		{name: "pending_events", dest: &stats.PendingEvents, query: "SELECT COUNT(*) FROM order_outbox WHERE status IN (0,2)"},
		{name: "dead_events", dest: &stats.DeadEvents, query: "SELECT COUNT(*) FROM order_outbox WHERE status = 3"},
	}

	for _, item := range queries {
		if err := db.QueryRowContext(ctx, item.query, item.args...).Scan(item.dest); err != nil {
			return stats, fmt.Errorf("%s: %w", item.name, err)
		}
	}
	return stats, nil
}

func fetchAdminUserTotal(r *http.Request, svcCtx *svc.ServiceContext) int64 {
	baseURL := strings.TrimRight(svcCtx.Config.AuthServiceBaseURL, "/")
	if baseURL == "" {
		return 0
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/admin/users?page=1&page_size=1", nil)
	if err != nil {
		return 0
	}
	if auth := r.Header.Get("Authorization"); auth != "" {
		req.Header.Set("Authorization", auth)
	}
	if userAgent := r.UserAgent(); userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		req.Header.Set("X-Forwarded-For", forwardedFor)
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		req.Header.Set("X-Real-IP", realIP)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logx.WithContext(r.Context()).Errorf("dashboard auth user total fetch failed: %v", err)
		return 0
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logx.WithContext(r.Context()).Errorf("dashboard auth user total fetch returned status=%d", resp.StatusCode)
		return 0
	}

	var payload struct {
		Total int64 `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		logx.WithContext(r.Context()).Errorf("dashboard auth user total decode failed: %v", err)
		return 0
	}
	return payload.Total
}
