package handler

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"flash-mall/app/common/orderstatus"
	"flash-mall/app/entry/api/internal/svc"
	"flash-mall/app/entry/api/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func MerchantMeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, err := extractAuthIdentity(r.Context())
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err := ensureMerchantBaseSchema(r.Context(), db); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		rows, err := db.QueryContext(r.Context(), `SELECT m.id, m.name, mu.role, m.status
FROM merchant_user mu
JOIN merchant m ON m.id = mu.merchant_id
WHERE mu.user_id = ? AND mu.status = 1
ORDER BY mu.id ASC`, identity.UserID)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer func() { _ = rows.Close() }()
		items := make([]map[string]any, 0)
		for rows.Next() {
			var merchantID int64
			var name string
			var role string
			var status int64
			if err := rows.Scan(&merchantID, &name, &role, &status); err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			items = append(items, map[string]any{"merchant_id": merchantID, "name": name, "role": role, "status": status})
		}
		httpx.OkJsonCtx(r.Context(), w, map[string]any{"items": items})
	}
}

func MerchantApplyCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, err := extractAuthIdentity(r.Context())
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		var req struct {
			MerchantName string `json:"merchant_name"`
			ContactPhone string `json:"contact_phone,optional"` //nolint:staticcheck
		}
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.MerchantName = strings.TrimSpace(req.MerchantName)
		req.ContactPhone = strings.TrimSpace(req.ContactPhone)
		if req.MerchantName == "" {
			writeBadRequest(w, "merchant_name required")
			return
		}
		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err := ensureMerchantBaseSchema(r.Context(), db); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		var activeApplyID int64
		err = db.QueryRowContext(r.Context(), "SELECT id FROM merchant_apply WHERE user_id = ? AND status = 0 ORDER BY id DESC LIMIT 1", identity.UserID).Scan(&activeApplyID)
		if err == nil {
			httpx.OkJsonCtx(r.Context(), w, map[string]any{"apply_id": activeApplyID, "status": "pending"})
			return
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		result, err := db.ExecContext(r.Context(), "INSERT INTO merchant_apply (user_id, merchant_name, contact_phone, status) VALUES (?, ?, ?, 0)", identity.UserID, req.MerchantName, req.ContactPhone)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		applyID, _ := result.LastInsertId()
		httpx.OkJsonCtx(r.Context(), w, map[string]any{"apply_id": applyID, "status": "pending"})
	}
}

func AdminMerchantApplyListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err := ensureMerchantBaseSchema(r.Context(), db); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		rows, err := db.QueryContext(r.Context(), `SELECT id, user_id, merchant_name, contact_phone, status, merchant_id, audit_remark, operator_id, COALESCE(create_time,''), COALESCE(audit_time,'')
FROM merchant_apply
ORDER BY id DESC
LIMIT 100`)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer func() { _ = rows.Close() }()
		items := make([]map[string]any, 0)
		for rows.Next() {
			var id, userID, status, merchantID, operatorID int64
			var name, phone, remark, createTime, auditTime string
			if err := rows.Scan(&id, &userID, &name, &phone, &status, &merchantID, &remark, &operatorID, &createTime, &auditTime); err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			items = append(items, map[string]any{"id": id, "user_id": userID, "merchant_name": name, "contact_phone": phone, "status": status, "merchant_id": merchantID, "audit_remark": remark, "operator_id": operatorID, "create_time": createTime, "audit_time": auditTime})
		}
		httpx.OkJsonCtx(r.Context(), w, map[string]any{"items": items})
	}
}

func AdminMerchantApplyAuditHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ApplyId int64  `json:"apply_id"`
			Approve bool   `json:"approve"`
			Remark  string `json:"remark,optional"` //nolint:staticcheck
		}
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if req.ApplyId <= 0 {
			writeBadRequest(w, "apply_id required")
			return
		}
		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err := ensureMerchantBaseSchema(r.Context(), db); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer func() { _ = tx.Rollback() }()
		var userID int64
		var merchantName, contactPhone string
		var currentStatus int64
		if err := tx.QueryRowContext(r.Context(), "SELECT user_id, merchant_name, contact_phone, status FROM merchant_apply WHERE id = ? FOR UPDATE", req.ApplyId).Scan(&userID, &merchantName, &contactPhone, &currentStatus); err != nil {
			writeNotFound(w, "merchant apply not found")
			return
		}
		if currentStatus != 0 {
			writeConflict(w, "merchant apply already audited")
			return
		}
		operatorID := adminOperatorID(r)
		statusValue := int64(2)
		merchantID := int64(0)
		if req.Approve {
			statusValue = 1
			result, err := tx.ExecContext(r.Context(), "INSERT INTO merchant (name, owner_user_id, status, contact_phone) VALUES (?, ?, 1, ?)", merchantName, userID, contactPhone)
			if err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			merchantID, _ = result.LastInsertId()
			if _, err := tx.ExecContext(r.Context(), "INSERT INTO merchant_user (merchant_id, user_id, role, status) VALUES (?, ?, 'owner', 1) ON DUPLICATE KEY UPDATE role = VALUES(role), status = VALUES(status)", merchantID, userID); err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
		}
		if _, err := tx.ExecContext(r.Context(), "UPDATE merchant_apply SET status = ?, merchant_id = ?, audit_remark = ?, operator_id = ?, audit_time = NOW() WHERE id = ?", statusValue, merchantID, strings.TrimSpace(req.Remark), operatorID, req.ApplyId); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err := tx.Commit(); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, map[string]any{"apply_id": req.ApplyId, "merchant_id": merchantID, "status": statusValue})
	}
}

func MerchantDashboardStatsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return withMerchantAccess(svcCtx, func(w http.ResponseWriter, r *http.Request, db *sql.DB, merchantID int64) {
		var orderCount, paidCount, shipPending, refundPending int64
		var salesAmount int64
		_ = db.QueryRowContext(r.Context(), "SELECT COUNT(*), SUM(CASE WHEN status IN (1,3,4,5,6) THEN 1 ELSE 0 END), SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) FROM orders WHERE merchant_id = ?", orderstatus.Paid, merchantID).Scan(&orderCount, &paidCount, &shipPending)
		_ = db.QueryRowContext(r.Context(), "SELECT COALESCE(SUM(s.payable_amount_fen),0) FROM orders o JOIN order_price_snapshot s ON s.order_id = o.id WHERE o.merchant_id = ? AND o.status IN (1,3,4,5,6)", merchantID).Scan(&salesAmount)
		_ = db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM refund_order WHERE merchant_id = ? AND status IN (0,1)", merchantID).Scan(&refundPending)
		httpx.OkJsonCtx(r.Context(), w, map[string]any{"merchant_id": merchantID, "order_count": orderCount, "paid_order_count": paidCount, "ship_pending_count": shipPending, "refund_pending_count": refundPending, "sales_amount_fen": salesAmount})
	})
}

func MerchantProductListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return withMerchantAccess(svcCtx, func(w http.ResponseWriter, r *http.Request, db *sql.DB, merchantID int64) {
		req := types.AdminProductListReq{MerchantId: merchantID, Page: 1, PageSize: 50, Status: -1, PromotionStatus: -1}
		_ = httpx.Parse(r, &req)
		req.MerchantId = merchantID
		writeMerchantProductList(w, r, db, req)
	})
}

func MerchantProductCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return withMerchantAccess(svcCtx, func(w http.ResponseWriter, r *http.Request, db *sql.DB, merchantID int64) {
		var req types.AdminProductCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		req.ImageUrl = strings.TrimSpace(req.ImageUrl)
		if req.Name == "" || req.OriginPriceFen < 0 || req.SalePriceFen < 0 || req.StockAvailable < 0 || req.SupplierId <= 0 {
			writeBadRequest(w, "name, non-negative prices, stock and supplier_id are required")
			return
		}
		if req.SalePriceFen > req.OriginPriceFen {
			writeBadRequest(w, "sale_price_fen must be <= origin_price_fen")
			return
		}
		if req.Status == 0 {
			req.Status = 1
		}
		if req.Status != 1 && req.Status != 2 {
			writeBadRequest(w, "status must be 1 or 2")
			return
		}
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer func() { _ = tx.Rollback() }()
		supplierExists, err := adminActiveSupplierExists(r.Context(), tx, req.SupplierId)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if !supplierExists {
			writeNotFound(w, "active supplier not found")
			return
		}
		productID, err := nextAdminProductID(r.Context(), tx)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if _, err = tx.ExecContext(r.Context(),
			"INSERT INTO mall_product.product (id, merchant_id, name, image_url, stock, version, origin_price_fen, sale_price_fen, status, supplier_id) VALUES (?, ?, ?, ?, 0, 0, ?, ?, ?, ?)",
			productID, merchantID, req.Name, req.ImageUrl, req.OriginPriceFen, req.SalePriceFen, req.Status, req.SupplierId,
		); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if req.StockAvailable > 0 {
			if err = insertAdminProductStockBuckets(r.Context(), tx, productID, req.StockAvailable); err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
		}
		if err = tx.Commit(); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err := seedRedisStockShards(r.Context(), svcCtx, productID, req.StockAvailable); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		invalidateAdminCatalogCache(r.Context(), svcCtx)
		httpx.OkJsonCtx(r.Context(), w, types.AdminProductCreateResp{ProductId: productID})
	})
}

func MerchantOrderListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return withMerchantAccess(svcCtx, func(w http.ResponseWriter, r *http.Request, db *sql.DB, merchantID int64) {
		req := types.AdminOrderListReq{MerchantId: merchantID, Page: 1, PageSize: 50, Status: -1}
		_ = httpx.Parse(r, &req)
		req.MerchantId = merchantID
		writeMerchantOrderList(w, r, db, req)
	})
}

func MerchantRefundListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return withMerchantAccess(svcCtx, func(w http.ResponseWriter, r *http.Request, db *sql.DB, merchantID int64) {
		req := types.AdminRefundListReq{MerchantId: merchantID, Page: 1, PageSize: 50, Status: -1}
		_ = httpx.Parse(r, &req)
		req.MerchantId = merchantID
		writeMerchantRefundList(w, r, db, req)
	})
}

func MerchantShipOrderHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return withMerchantAccess(svcCtx, func(w http.ResponseWriter, r *http.Request, db *sql.DB, merchantID int64) {
		var req types.ShipOrderReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		orderID := strings.TrimSpace(req.OrderId)
		if orderID == "" {
			writeBadRequest(w, "order_id required")
			return
		}
		result, err := db.ExecContext(r.Context(), "UPDATE orders SET status = ?, shipped_at = NOW() WHERE id = ? AND merchant_id = ? AND status = ?", orderstatus.Shipped, orderID, merchantID, orderstatus.Paid)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			writeConflict(w, "order not found for merchant or not in paid status")
			return
		}
		_, _ = db.ExecContext(r.Context(), "INSERT INTO order_status_log (order_id, from_status, to_status, operator_id, remark) VALUES (?, ?, ?, ?, ?)", orderID, orderstatus.Paid, orderstatus.Shipped, merchantID, "merchant ship order")
		httpx.OkJsonCtx(r.Context(), w, types.ShipOrderResp{OrderId: orderID, Status: "shipped"})
	})
}

func withMerchantAccess(svcCtx *svc.ServiceContext, next func(http.ResponseWriter, *http.Request, *sql.DB, int64)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, err := extractAuthIdentity(r.Context())
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err := ensureMerchantSchema(r.Context(), db); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err := ensureProductImageColumn(r.Context(), db); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		merchantID := parseOptionalInt64(r.URL.Query().Get("merchant_id"))
		if merchantID <= 0 {
			merchantID, err = firstAccessibleMerchantID(r, db, identity.UserID)
			if err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
		}
		role, _ := r.Context().Value("role").(string)
		allowed, err := userCanAccessMerchant(r.Context(), db, identity.UserID, merchantID, role)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if !allowed {
			writeForbidden(w, "merchant access denied")
			return
		}
		next(w, r, db, merchantID)
	}
}

func firstAccessibleMerchantID(r *http.Request, db *sql.DB, userID int64) (int64, error) {
	role, _ := r.Context().Value("role").(string)
	if role == "admin" {
		return defaultMerchantID, nil
	}
	var merchantID int64
	err := db.QueryRowContext(r.Context(), "SELECT merchant_id FROM merchant_user WHERE user_id = ? AND status = 1 ORDER BY id ASC LIMIT 1", userID).Scan(&merchantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("merchant access required")
		}
		return 0, err
	}
	return merchantID, nil
}

func parseOptionalInt64(value string) int64 {
	parsed, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return parsed
}

func writeForbidden(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(fmt.Sprintf(`{"error":%q}`, message)))
}

func writeMerchantProductList(w http.ResponseWriter, r *http.Request, db *sql.DB, req types.AdminProductListReq) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 100 {
		req.PageSize = 50
	}
	where := "p.merchant_id = ?"
	args := []any{req.MerchantId}
	if req.Status >= 0 {
		where += " AND p.status = ?"
		args = append(args, req.Status)
	}
	if keyword := strings.TrimSpace(req.Keyword); keyword != "" {
		where += " AND p.name LIKE ?"
		args = append(args, "%"+keyword+"%")
	}
	var total int64
	if err := db.QueryRowContext(r.Context(), fmt.Sprintf("SELECT COUNT(*) FROM mall_product.product p WHERE %s", where), args...).Scan(&total); err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}
	queryArgs := append(append([]any{}, args...), req.PageSize, (req.Page-1)*req.PageSize)
	rows, err := db.QueryContext(r.Context(), fmt.Sprintf(`SELECT p.id, p.merchant_id, COALESCE(m.name, ''), p.name, COALESCE(p.image_url, ''), p.origin_price_fen, p.sale_price_fen, p.supplier_id, COALESCE(s.name, ''),
       COALESCE(stock.stock_available, 0), p.status
FROM mall_product.product p
LEFT JOIN merchant m ON m.id = p.merchant_id
LEFT JOIN mall_product.supplier s ON s.id = p.supplier_id
LEFT JOIN (
  SELECT product_id, COALESCE(SUM(stock), 0) AS stock_available
  FROM mall_product.product_stock_bucket
  GROUP BY product_id
) stock ON stock.product_id = p.id
WHERE %s
ORDER BY p.id DESC LIMIT ? OFFSET ?`, where), queryArgs...)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}
	defer func() { _ = rows.Close() }()
	items := make([]types.AdminProductItem, 0)
	for rows.Next() {
		var item types.AdminProductItem
		if err := rows.Scan(&item.ProductId, &item.MerchantId, &item.MerchantName, &item.Name, &item.ImageUrl, &item.OriginPriceFen, &item.SalePriceFen, &item.SupplierId, &item.SupplierName, &item.StockAvailable, &item.Status); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		item.StatusText = adminProductStatusText(item.Status)
		items = append(items, item)
	}
	httpx.OkJsonCtx(r.Context(), w, types.AdminProductListResp{Items: items, Total: total})
}

func writeMerchantOrderList(w http.ResponseWriter, r *http.Request, db *sql.DB, req types.AdminOrderListReq) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 100 {
		req.PageSize = 50
	}
	where := "o.merchant_id = ?"
	args := []any{req.MerchantId}
	if req.Status >= 0 {
		where += " AND o.status = ?"
		args = append(args, req.Status)
	}
	var total int64
	if err := db.QueryRowContext(r.Context(), fmt.Sprintf("SELECT COUNT(*) FROM orders o WHERE %s", where), args...).Scan(&total); err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}
	queryArgs := append(append([]any{}, args...), req.PageSize, (req.Page-1)*req.PageSize)
	rows, err := db.QueryContext(r.Context(), fmt.Sprintf(`SELECT o.id, o.user_id, o.merchant_id, COALESCE(m.name, ''), o.product_id, COALESCE(s.product_name, ''), o.amount, o.status, COALESCE(s.payable_amount_fen, 0), COALESCE(o.create_time, '')
FROM orders o
LEFT JOIN order_price_snapshot s ON s.order_id = o.id
LEFT JOIN merchant m ON m.id = o.merchant_id
WHERE %s
ORDER BY o.create_time DESC LIMIT ? OFFSET ?`, where), queryArgs...)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}
	defer func() { _ = rows.Close() }()
	items := make([]types.AdminOrderListItem, 0)
	for rows.Next() {
		var item types.AdminOrderListItem
		if err := rows.Scan(&item.OrderId, &item.UserId, &item.MerchantId, &item.MerchantName, &item.ProductId, &item.ProductName, &item.Amount, &item.Status, &item.PayableAmountFen, &item.CreateTime); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		item.StatusText = orderStatusText(item.Status)
		items = append(items, item)
	}
	httpx.OkJsonCtx(r.Context(), w, types.AdminOrderListResp{Items: items, Total: total})
}

func writeMerchantRefundList(w http.ResponseWriter, r *http.Request, db *sql.DB, req types.AdminRefundListReq) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 100 {
		req.PageSize = 50
	}
	where := "r.merchant_id = ?"
	args := []any{req.MerchantId}
	if req.Status >= 0 {
		where += " AND r.status = ?"
		args = append(args, req.Status)
	}
	var total int64
	if err := db.QueryRowContext(r.Context(), fmt.Sprintf("SELECT COUNT(*) FROM refund_order r WHERE %s", where), args...).Scan(&total); err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}
	queryArgs := append(append([]any{}, args...), req.PageSize, (req.Page-1)*req.PageSize)
	rows, err := db.QueryContext(r.Context(), fmt.Sprintf(`SELECT r.id, r.order_id, r.payment_order_id, r.user_id, r.merchant_id, COALESCE(m.name, ''), r.product_id, r.refund_amount_fen, r.status,
       r.reason, r.audit_remark, r.operator_id, COALESCE(r.request_time,''), COALESCE(r.audit_time,''), COALESCE(r.finish_time,'')
FROM refund_order r
LEFT JOIN merchant m ON m.id = r.merchant_id
WHERE %s
ORDER BY r.create_time DESC LIMIT ? OFFSET ?`, where), queryArgs...)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}
	defer func() { _ = rows.Close() }()
	items := make([]types.AdminRefundItem, 0)
	for rows.Next() {
		var item types.AdminRefundItem
		if err := rows.Scan(&item.RefundId, &item.OrderId, &item.PaymentOrderId, &item.UserId, &item.MerchantId, &item.MerchantName, &item.ProductId,
			&item.RefundAmountFen, &item.Status, &item.Reason, &item.AuditRemark, &item.OperatorId,
			&item.RequestTime, &item.AuditTime, &item.FinishTime); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		item.StatusText = refundStatusText(item.Status)
		items = append(items, item)
	}
	httpx.OkJsonCtx(r.Context(), w, types.AdminRefundListResp{Items: items, Total: total})
}
