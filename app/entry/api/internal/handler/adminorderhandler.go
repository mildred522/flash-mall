package handler

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"flash-mall/app/common/orderstatus"
	"flash-mall/app/entry/api/internal/svc"
	"flash-mall/app/entry/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func AdminOrderListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminOrderListReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err := ensureOrderMerchantSchema(r.Context(), db); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		if req.Page <= 0 {
			req.Page = 1
		}
		if req.PageSize <= 0 || req.PageSize > 100 {
			req.PageSize = 20
		}
		offset := (req.Page - 1) * req.PageSize

		where := "1=1"
		args := []interface{}{}
		if req.Status >= 0 {
			where += " AND o.status = ?"
			args = append(args, req.Status)
		}
		if req.UserId > 0 {
			where += " AND o.user_id = ?"
			args = append(args, req.UserId)
		}
		if req.MerchantId > 0 {
			where += " AND o.merchant_id = ?"
			args = append(args, req.MerchantId)
		}
		if req.ProductId > 0 {
			where += " AND o.product_id = ?"
			args = append(args, req.ProductId)
		}
		if productName := strings.TrimSpace(req.ProductName); productName != "" {
			where += " AND s.product_name LIKE ?"
			args = append(args, "%"+productName+"%")
		}
		if createdFrom := strings.TrimSpace(req.CreatedFrom); createdFrom != "" {
			where += " AND o.create_time >= ?"
			args = append(args, normalizeAdminDateTimeLower(createdFrom))
		}
		if createdTo := strings.TrimSpace(req.CreatedTo); createdTo != "" {
			where += " AND o.create_time <= ?"
			args = append(args, normalizeAdminDateTimeUpper(createdTo))
		}
		if req.OrderId != "" {
			where += " AND o.id = ?"
			args = append(args, req.OrderId)
		}

		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM orders o LEFT JOIN order_price_snapshot s ON s.order_id = o.id WHERE %s", where)
		var total int64
		if err := db.QueryRowContext(r.Context(), countQuery, args...).Scan(&total); err != nil {
			logx.WithContext(r.Context()).Errorf("admin order count query failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		query := fmt.Sprintf(
			`SELECT o.id, o.user_id, o.merchant_id, COALESCE(m.name, ''), o.product_id, COALESCE(s.product_name, ''), o.amount, o.status, COALESCE(s.payable_amount_fen, 0), COALESCE(o.create_time, '')
			 FROM orders o
			 LEFT JOIN order_price_snapshot s ON s.order_id = o.id
			 LEFT JOIN merchant m ON m.id = o.merchant_id
			 WHERE %s ORDER BY o.create_time DESC LIMIT ? OFFSET ?`, where)
		args = append(args, req.PageSize, offset)

		rows, err := db.QueryContext(r.Context(), query, args...)
		if err != nil {
			logx.WithContext(r.Context()).Errorf("admin order list query failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer func() { _ = rows.Close() }()

		items := make([]types.AdminOrderListItem, 0)
		for rows.Next() {
			var item types.AdminOrderListItem
			if err := rows.Scan(&item.OrderId, &item.UserId, &item.MerchantId, &item.MerchantName, &item.ProductId, &item.ProductName, &item.Amount, &item.Status, &item.PayableAmountFen, &item.CreateTime); err != nil {
				logx.WithContext(r.Context()).Errorf("admin order list scan failed: %v", err)
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			item.StatusText = orderStatusText(item.Status)
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			logx.WithContext(r.Context()).Errorf("admin order list rows failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		httpx.OkJsonCtx(r.Context(), w, types.AdminOrderListResp{Items: items, Total: total})
	}
}

func normalizeAdminDateTimeLower(value string) string {
	value = strings.TrimSpace(value)
	if len(value) == len("2006-01-02") {
		return value + " 00:00:00"
	}
	return value
}

func normalizeAdminDateTimeUpper(value string) string {
	value = strings.TrimSpace(value)
	if len(value) == len("2006-01-02") {
		return value + " 23:59:59"
	}
	return value
}

func AdminOrderDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orderId := r.URL.Query().Get("order_id")
		if orderId == "" {
			writeBadRequest(w, "order_id required")
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err := ensureOrderMerchantSchema(r.Context(), db); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		var item types.OrderDetailResp
		err = db.QueryRowContext(r.Context(),
			`SELECT o.id, o.user_id, o.merchant_id, COALESCE(m.name, ''), o.product_id, COALESCE(s.product_name,''), o.amount, o.status,
			        COALESCE(s.origin_unit_price_fen,0), COALESCE(s.sale_unit_price_fen,0),
			        COALESCE(s.payable_amount_fen,0), COALESCE(s.discount_amount_fen,0),
			        COALESCE(s.promotion_type,''), COALESCE(s.promotion_tag,''),
			        COALESCE(p.id,''), COALESCE(p.status,0), COALESCE(o.create_time,'')
			 FROM orders o
			 LEFT JOIN order_price_snapshot s ON s.order_id = o.id
			 LEFT JOIN merchant m ON m.id = o.merchant_id
			 LEFT JOIN payment_order p ON p.order_id = o.id
			 WHERE o.id = ?`, orderId,
		).Scan(&item.OrderId, &item.UserId, &item.MerchantId, &item.MerchantName, &item.ProductId, &item.ProductName, &item.Amount, &item.Status,
			&item.OriginUnitPriceFen, &item.SaleUnitPriceFen, &item.PayableAmountFen, &item.DiscountAmountFen,
			&item.PromotionType, &item.PromotionTag, &item.PaymentOrderId, &item.PaymentStatus, &item.CreateTime)
		if err != nil {
			writeNotFound(w, "order not found")
			return
		}
		item.StatusText = orderStatusText(item.Status)
		item.PaymentStatusText = paymentStatusText(item.PaymentStatus)
		httpx.OkJsonCtx(r.Context(), w, item)
	}
}

func AdminOrderStatusLogHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminOrderStatusLogReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.OrderId = strings.TrimSpace(req.OrderId)
		if req.OrderId == "" {
			writeBadRequest(w, "order_id required")
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		rows, err := db.QueryContext(r.Context(),
			`SELECT id, order_id, from_status, to_status, operator_id, remark, COALESCE(create_time, '')
			 FROM order_status_log
			 WHERE order_id = ?
			 ORDER BY id ASC`,
			req.OrderId)
		if err != nil {
			logx.WithContext(r.Context()).Errorf("admin order status log query failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer func() { _ = rows.Close() }()

		items := make([]types.AdminOrderStatusLogItem, 0)
		for rows.Next() {
			var item types.AdminOrderStatusLogItem
			if err := rows.Scan(&item.Id, &item.OrderId, &item.FromStatus, &item.ToStatus, &item.OperatorId, &item.Remark, &item.CreateTime); err != nil {
				logx.WithContext(r.Context()).Errorf("admin order status log scan failed: %v", err)
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			item.FromStatusText = orderStatusText(item.FromStatus)
			item.ToStatusText = orderStatusText(item.ToStatus)
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			logx.WithContext(r.Context()).Errorf("admin order status log rows failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		httpx.OkJsonCtx(r.Context(), w, types.AdminOrderStatusLogResp{Items: items})
	}
}

func AdminShipOrderHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ShipOrderReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer func() { _ = tx.Rollback() }()

		result, err := tx.ExecContext(r.Context(),
			"UPDATE orders SET status = ?, shipped_at = NOW() WHERE id = ? AND status = ?",
			orderstatus.Shipped, req.OrderId, orderstatus.Paid)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		rows, err := result.RowsAffected()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if rows == 0 {
			recordAdminAuditFailure(r, svcCtx, adminAuditOrderShipped, fmt.Sprintf("order:%s operator:%d reason:%s", req.OrderId, adminOperatorID(r), adminAuditReasonNotPaidStatus))
			writeConflict(w, "order not in paid status")
			return
		}
		operatorID := adminOperatorID(r)
		if _, err = tx.ExecContext(r.Context(),
			"INSERT INTO order_status_log (order_id, from_status, to_status, operator_id, remark) VALUES (?, ?, ?, ?, 'admin shipped')",
			req.OrderId, orderstatus.Paid, orderstatus.Shipped, operatorID); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err = tx.Commit(); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		recordAdminAuditEvent(r, svcCtx, adminAuditOrderShipped, fmt.Sprintf("order:%s operator:%d", req.OrderId, operatorID))
		httpx.OkJsonCtx(r.Context(), w, types.ShipOrderResp{OrderId: req.OrderId, Status: orderStatusText(orderstatus.Shipped)})
	}
}

func AdminCloseOrderHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CancelOrderReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.OrderId = strings.TrimSpace(req.OrderId)
		if req.OrderId == "" {
			writeBadRequest(w, "order_id required")
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer func() { _ = tx.Rollback() }()

		var currentStatus int64
		var productID int64
		var amount int64
		err = tx.QueryRowContext(r.Context(), "SELECT status, product_id, amount FROM orders WHERE id = ? FOR UPDATE", req.OrderId).Scan(&currentStatus, &productID, &amount)
		if err != nil {
			if err == sql.ErrNoRows {
				recordAdminAuditFailure(r, svcCtx, adminAuditOrderClosed, fmt.Sprintf("order:%s operator:%d reason:%s", req.OrderId, adminOperatorID(r), adminAuditReasonNotFound))
				writeNotFound(w, "order not found")
				return
			}
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if !orderstatus.CanPay(currentStatus) {
			recordAdminAuditFailure(r, svcCtx, adminAuditOrderClosed, fmt.Sprintf("order:%s operator:%d reason:%s", req.OrderId, adminOperatorID(r), adminAuditReasonInvalidStatus))
			writeConflict(w, "order cannot be closed")
			return
		}

		result, err := tx.ExecContext(r.Context(),
			"UPDATE orders SET status = ? WHERE id = ? AND status = ?",
			orderstatus.Closed, req.OrderId, orderstatus.PendingPayment)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		rows, err := result.RowsAffected()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if rows == 0 {
			recordAdminAuditFailure(r, svcCtx, adminAuditOrderClosed, fmt.Sprintf("order:%s operator:%d reason:%s", req.OrderId, adminOperatorID(r), adminAuditReasonInvalidStatus))
			writeConflict(w, "order cannot be closed")
			return
		}

		operatorID := adminOperatorID(r)
		reason := strings.TrimSpace(req.Reason)
		if reason == "" {
			reason = "admin close"
		}
		if _, err = tx.ExecContext(r.Context(),
			"INSERT INTO order_status_log (order_id, from_status, to_status, operator_id, remark) VALUES (?, ?, ?, ?, ?)",
			req.OrderId, orderstatus.PendingPayment, orderstatus.Closed, operatorID, "admin closed: "+reason); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err := compensateClosedOrderInventory(r.Context(), svcCtx, productID, amount, req.OrderId); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err = tx.Commit(); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		recordAdminAuditEvent(r, svcCtx, adminAuditOrderClosed, fmt.Sprintf("order:%s operator:%d reason:%s", req.OrderId, operatorID, reason))
		httpx.OkJsonCtx(r.Context(), w, types.CancelOrderResp{OrderId: req.OrderId, Status: orderStatusText(orderstatus.Closed)})
	}
}

func AdminRefundOrderHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.RefundOrderReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		var currentStatus int64
		var productID int64
		var amount int64
		err = db.QueryRowContext(r.Context(), "SELECT status, product_id, amount FROM orders WHERE id = ?", req.OrderId).Scan(&currentStatus, &productID, &amount)
		if err != nil {
			recordAdminAuditFailure(r, svcCtx, adminAuditOrderRefunded, fmt.Sprintf("order:%s operator:%d reason:%s", req.OrderId, adminOperatorID(r), adminAuditReasonNotFound))
			writeNotFound(w, "order not found")
			return
		}
		if !orderstatus.CanRequestRefund(currentStatus) {
			recordAdminAuditFailure(r, svcCtx, adminAuditOrderRefunded, fmt.Sprintf("order:%s operator:%d reason:%s status:%d", req.OrderId, adminOperatorID(r), adminAuditReasonInvalidStatus, currentStatus))
			writeConflict(w, "order cannot be refunded")
			return
		}

		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer func() { _ = tx.Rollback() }()

		result, err := tx.ExecContext(r.Context(),
			"UPDATE orders SET status = ?, refund_requested_at = NOW(), refunded_at = NOW() WHERE id = ? AND status = ?",
			orderstatus.Refunded, req.OrderId, currentStatus)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		rows, err := result.RowsAffected()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if rows == 0 {
			recordAdminAuditFailure(r, svcCtx, adminAuditOrderRefunded, fmt.Sprintf("order:%s operator:%d reason:%s", req.OrderId, adminOperatorID(r), adminAuditReasonStatusChanged))
			writeConflict(w, "status changed concurrently")
			return
		}
		operatorID := adminOperatorID(r)
		if _, err = tx.ExecContext(r.Context(),
			"INSERT INTO order_status_log (order_id, from_status, to_status, operator_id, remark) VALUES (?, ?, ?, ?, ?)",
			req.OrderId, currentStatus, orderstatus.Refunded, operatorID, "admin refund: "+req.Reason); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err := compensateClosedOrderInventory(r.Context(), svcCtx, productID, amount, req.OrderId); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err = tx.Commit(); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		recordAdminAuditEvent(r, svcCtx, adminAuditOrderRefunded, fmt.Sprintf("order:%s operator:%d reason:%s", req.OrderId, operatorID, strings.TrimSpace(req.Reason)))
		httpx.OkJsonCtx(r.Context(), w, types.RefundOrderResp{OrderId: req.OrderId, Status: orderStatusText(orderstatus.Refunded)})
	}
}

func adminOperatorID(r *http.Request) int64 {
	if userID, ok := parseUserIDClaim(r.Context().Value("user_id")); ok {
		return userID
	}
	if userID, ok := parseUserIDClaim(r.Context().Value("sub")); ok {
		return userID
	}
	return 0
}
