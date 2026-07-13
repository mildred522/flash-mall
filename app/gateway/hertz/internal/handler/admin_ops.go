package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/orderstatus"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func AdminDashboardStatsHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		db, err := orderDB(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order datasource unavailable", err))
			return
		}
		stats, err := loadAdminDashboardStats(ctx, db)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "dashboard stats query failed", err))
			return
		}
		stats.TotalUsers = fetchGatewayAdminUserTotal(ctx, c, svcCtx)
		ok(ctx, c, stats)
	}
}

func AdminReconciliationListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		req, err := adminReconciliationQueryFromRequest(c)
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, err)
			return
		}
		db, err := orderDB(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order datasource unavailable", err))
			return
		}
		resp, err := loadAdminReconciliationIssues(ctx, db, req)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "reconciliation query failed", err))
			return
		}
		ok(ctx, c, resp)
	}
}

func AdminReconciliationScanHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		db, err := orderDB(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order datasource unavailable", err))
			return
		}
		inserted, err := scanGatewayReconciliationIssues(ctx, db)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "reconciliation scan failed", err))
			return
		}
		ok(ctx, c, map[string]any{"inserted": inserted})
	}
}

func AdminEventListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		req, err := adminEventQueryFromRequest(c)
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, err)
			return
		}
		db, err := orderDB(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order datasource unavailable", err))
			return
		}
		resp, err := loadAdminEvents(ctx, db, req)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "event query failed", err))
			return
		}
		ok(ctx, c, resp)
	}
}

func AdminEventRetryHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req AdminEventRetryReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid event retry request"))
			return
		}
		req.EventID = strings.TrimSpace(req.EventID)
		if req.EventID == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "event_id is required"))
			return
		}
		db, err := orderDB(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order datasource unavailable", err))
			return
		}
		if _, err = db.ExecContext(ctx, "UPDATE order_outbox SET status = 0, next_retry_at = NOW(), last_error = '' WHERE event_id = ?", req.EventID); err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "event retry failed", err))
			return
		}
		ok(ctx, c, AdminEventRetryResp{EventID: req.EventID, Status: "pending"})
	}
}

func loadAdminDashboardStats(ctx context.Context, db *sql.DB) (AdminDashboardStats, error) {
	var stats AdminDashboardStats
	queries := []struct {
		dest  *int64
		query string
		args  []any
	}{
		{dest: &stats.TotalOrders, query: "SELECT COUNT(*) FROM orders"},
		{dest: &stats.TotalRevenueFen, query: `SELECT COALESCE(SUM(s.payable_amount_fen), 0) FROM orders o JOIN order_price_snapshot s ON s.order_id = o.id WHERE o.status IN (?,?,?)`, args: []any{orderstatus.Paid, orderstatus.Shipped, orderstatus.Completed}},
		{dest: &stats.TotalProducts, query: "SELECT COUNT(*) FROM mall_product.product"},
		{dest: &stats.TotalSuppliers, query: "SELECT COUNT(*) FROM mall_product.supplier"},
		{dest: &stats.TotalPromotions, query: "SELECT COUNT(*) FROM mall_product.promotion_rule"},
		{dest: &stats.ActivePromotions, query: "SELECT COUNT(*) FROM mall_product.promotion_rule WHERE status = 1 AND (starts_at IS NULL OR starts_at <= NOW()) AND (ends_at IS NULL OR ends_at >= NOW())"},
		{dest: &stats.LowStockProducts, query: `SELECT COUNT(*) FROM (SELECT p.id, COALESCE(snap.available, p.stock, 0) AS stock_available FROM mall_product.product p LEFT JOIN mall_product.product_stock_snapshot snap ON snap.product_id = p.id WHERE p.status = 1 HAVING stock_available > 0 AND stock_available <= 100) low_stock`},
		{dest: &stats.OutOfStockProducts, query: `SELECT COUNT(*) FROM (SELECT p.id, COALESCE(snap.available, p.stock, 0) AS stock_available FROM mall_product.product p LEFT JOIN mall_product.product_stock_snapshot snap ON snap.product_id = p.id WHERE p.status = 1 HAVING stock_available = 0) out_of_stock`},
		{dest: &stats.PendingOrders, query: "SELECT COUNT(*) FROM orders WHERE status = ?", args: []any{orderstatus.PendingPayment}},
		{dest: &stats.PaidOrders, query: "SELECT COUNT(*) FROM orders WHERE status = ?", args: []any{orderstatus.Paid}},
		{dest: &stats.ShippedOrders, query: "SELECT COUNT(*) FROM orders WHERE status = ?", args: []any{orderstatus.Shipped}},
		{dest: &stats.CompletedOrders, query: "SELECT COUNT(*) FROM orders WHERE status = ?", args: []any{orderstatus.Completed}},
		{dest: &stats.RefundRequested, query: "SELECT COUNT(*) FROM orders WHERE status = ?", args: []any{orderstatus.RefundRequested}},
		{dest: &stats.RefundedOrders, query: "SELECT COUNT(*) FROM orders WHERE status = ?", args: []any{orderstatus.Refunded}},
		{dest: &stats.OpenReconIssues, query: "SELECT COUNT(*) FROM reconciliation_issue WHERE status = 0"},
		{dest: &stats.PendingEvents, query: "SELECT COUNT(*) FROM order_outbox WHERE status IN (0,2)"},
		{dest: &stats.DeadEvents, query: "SELECT COUNT(*) FROM order_outbox WHERE status = 3"},
	}
	for _, item := range queries {
		if err := db.QueryRowContext(ctx, item.query, item.args...).Scan(item.dest); err != nil {
			return stats, err
		}
	}
	return stats, nil
}

func fetchGatewayAdminUserTotal(ctx context.Context, c *app.RequestContext, svcCtx *svc.ServiceContext) int64 {
	baseURL := strings.TrimRight(svcCtx.Config.AuthServiceBaseURL, "/")
	if baseURL == "" {
		return 0
	}
	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, baseURL+"/api/admin/users?page=1&page_size=1", nil)
	if err != nil {
		return 0
	}
	copyHeaderIfPresent(c, req, "Authorization")
	copyHeaderIfPresent(c, req, "User-Agent")
	copyHeaderIfPresent(c, req, "X-Forwarded-For")
	copyHeaderIfPresent(c, req, "X-Real-IP")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return 0
	}
	var payload struct {
		Total int64 `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0
	}
	return payload.Total
}

func copyHeaderIfPresent(c *app.RequestContext, req *http.Request, name string) {
	if value := string(c.GetHeader(name)); value != "" {
		req.Header.Set(name, value)
	}
}

func adminReconciliationQueryFromRequest(c *app.RequestContext) (AdminReconciliationReq, error) {
	page, err := parseInt64Default(c.Query("page"), 1)
	if err != nil {
		return AdminReconciliationReq{}, apperror.New(apperror.CodeInvalidArgument, "page must be numeric")
	}
	pageSize, err := parseInt64Default(c.Query("page_size"), 20)
	if err != nil {
		return AdminReconciliationReq{}, apperror.New(apperror.CodeInvalidArgument, "page_size must be numeric")
	}
	status, err := parseInt64Default(c.Query("status"), -1)
	if err != nil {
		return AdminReconciliationReq{}, apperror.New(apperror.CodeInvalidArgument, "status must be numeric")
	}
	return AdminReconciliationReq{Page: normalizePage(page), PageSize: normalizeAdminPageSize(pageSize), Status: status, IssueType: strings.TrimSpace(c.Query("issue_type")), OrderID: strings.TrimSpace(c.Query("order_id"))}, nil
}

func loadAdminReconciliationIssues(ctx context.Context, db *sql.DB, req AdminReconciliationReq) (AdminReconciliationResp, error) {
	where := "1=1"
	args := []any{}
	if req.Status >= 0 {
		where += " AND status = ?"
		args = append(args, req.Status)
	}
	if req.IssueType != "" {
		where += " AND issue_type = ?"
		args = append(args, req.IssueType)
	}
	if req.OrderID != "" {
		where += " AND order_id = ?"
		args = append(args, req.OrderID)
	}
	var total int64
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM reconciliation_issue WHERE "+where, args...).Scan(&total); err != nil {
		return AdminReconciliationResp{}, err
	}
	queryArgs := append(append([]any{}, args...), req.PageSize, (req.Page-1)*req.PageSize)
	rows, err := db.QueryContext(ctx, `SELECT id, issue_type, order_id, payment_order_id, refund_order_id, expected_amount_fen,
       actual_amount_fen, severity, status, detail, COALESCE(DATE_FORMAT(create_time, '%Y-%m-%d %H:%i:%s'), '')
FROM reconciliation_issue
WHERE `+where+`
ORDER BY id DESC
LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return AdminReconciliationResp{}, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]AdminReconciliationItem, 0)
	for rows.Next() {
		var item AdminReconciliationItem
		if err := rows.Scan(&item.ID, &item.IssueType, &item.OrderID, &item.PaymentOrderID, &item.RefundOrderID, &item.ExpectedAmountFen, &item.ActualAmountFen, &item.Severity, &item.Status, &item.Detail, &item.CreateTime); err != nil {
			return AdminReconciliationResp{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return AdminReconciliationResp{}, err
	}
	return AdminReconciliationResp{Items: items, Total: total}, nil
}

func scanGatewayReconciliationIssues(ctx context.Context, db *sql.DB) (int64, error) {
	now := time.Now().Format("20060102150405")
	result, err := db.ExecContext(ctx, `INSERT INTO reconciliation_issue (issue_type, order_id, payment_order_id, expected_amount_fen, actual_amount_fen, severity, detail)
SELECT 'payment_amount_mismatch', o.id, p.id, COALESCE(s.payable_amount_fen,0), COALESCE(p.payable_amount_fen,0), 3, CONCAT('scan:', ?)
FROM orders o
LEFT JOIN order_price_snapshot s ON s.order_id = o.id
JOIN payment_order p ON p.order_id = o.id
WHERE p.status = 1 AND COALESCE(s.payable_amount_fen,0) <> COALESCE(p.payable_amount_fen,0)
  AND NOT EXISTS (
    SELECT 1 FROM reconciliation_issue i
    WHERE i.issue_type = 'payment_amount_mismatch' AND i.order_id = o.id AND i.status = 0
  )`, now)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func adminEventQueryFromRequest(c *app.RequestContext) (AdminEventListReq, error) {
	page, err := parseInt64Default(c.Query("page"), 1)
	if err != nil {
		return AdminEventListReq{}, apperror.New(apperror.CodeInvalidArgument, "page must be numeric")
	}
	pageSize, err := parseInt64Default(c.Query("page_size"), 20)
	if err != nil {
		return AdminEventListReq{}, apperror.New(apperror.CodeInvalidArgument, "page_size must be numeric")
	}
	status, err := parseInt64Default(c.Query("status"), -1)
	if err != nil {
		return AdminEventListReq{}, apperror.New(apperror.CodeInvalidArgument, "status must be numeric")
	}
	return AdminEventListReq{Page: normalizePage(page), PageSize: normalizeAdminPageSize(pageSize), Status: status, EventType: strings.TrimSpace(c.Query("event_type")), AggregateID: strings.TrimSpace(c.Query("aggregate_id"))}, nil
}

func loadAdminEvents(ctx context.Context, db *sql.DB, req AdminEventListReq) (AdminEventListResp, error) {
	where := "1=1"
	args := []any{}
	if req.Status >= 0 {
		where += " AND status = ?"
		args = append(args, req.Status)
	}
	if req.EventType != "" {
		where += " AND event_type = ?"
		args = append(args, req.EventType)
	}
	if req.AggregateID != "" {
		where += " AND aggregate_id = ?"
		args = append(args, req.AggregateID)
	}
	var total int64
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM order_outbox WHERE "+where, args...).Scan(&total); err != nil {
		return AdminEventListResp{}, err
	}
	queryArgs := append(append([]any{}, args...), req.PageSize, (req.Page-1)*req.PageSize)
	rows, err := db.QueryContext(ctx, `SELECT id, event_id, event_type, aggregate_id, status, attempt_count, last_error,
       COALESCE(DATE_FORMAT(create_time, '%Y-%m-%d %H:%i:%s'), ''), COALESCE(DATE_FORMAT(update_time, '%Y-%m-%d %H:%i:%s'), '')
FROM order_outbox
WHERE `+where+`
ORDER BY id DESC
LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return AdminEventListResp{}, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]AdminEventItem, 0)
	for rows.Next() {
		var item AdminEventItem
		if err := rows.Scan(&item.ID, &item.EventID, &item.EventType, &item.AggregateID, &item.Status, &item.AttemptCount, &item.LastError, &item.CreateTime, &item.UpdateTime); err != nil {
			return AdminEventListResp{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return AdminEventListResp{}, err
	}
	return AdminEventListResp{Items: items, Total: total}, nil
}
