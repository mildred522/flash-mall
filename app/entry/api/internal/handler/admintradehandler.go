package handler

import (
	"database/sql"
	"errors"
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

func AdminRefundListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminRefundListReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if req.Page <= 0 {
			req.Page = 1
		}
		if req.PageSize <= 0 || req.PageSize > 100 {
			req.PageSize = 20
		}
		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		where, args := "1=1", []any{}
		if req.Status >= 0 {
			where += " AND status = ?"
			args = append(args, req.Status)
		}
		if req.UserId > 0 {
			where += " AND user_id = ?"
			args = append(args, req.UserId)
		}
		if req.OrderId != "" {
			where += " AND order_id = ?"
			args = append(args, strings.TrimSpace(req.OrderId))
		}
		var total int64
		if err := db.QueryRowContext(r.Context(), fmt.Sprintf("SELECT COUNT(*) FROM refund_order WHERE %s", where), args...).Scan(&total); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		args = append(args, req.PageSize, (req.Page-1)*req.PageSize)
		rows, err := db.QueryContext(r.Context(), fmt.Sprintf(
			`SELECT id, order_id, payment_order_id, user_id, product_id, refund_amount_fen, status,
			        reason, audit_remark, operator_id, COALESCE(request_time,''), COALESCE(audit_time,''), COALESCE(finish_time,'')
			   FROM refund_order WHERE %s ORDER BY create_time DESC LIMIT ? OFFSET ?`, where), args...)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer func() { _ = rows.Close() }()
		items := make([]types.AdminRefundItem, 0)
		for rows.Next() {
			var item types.AdminRefundItem
			if err := rows.Scan(&item.RefundId, &item.OrderId, &item.PaymentOrderId, &item.UserId, &item.ProductId,
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
}

func AdminRefundAuditHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminRefundAuditReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.RefundId = strings.TrimSpace(req.RefundId)
		if req.RefundId == "" {
			writeBadRequest(w, "refund_id required")
			return
		}
		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		newStatus := int64(3)
		orderStatus := orderstatus.Paid
		finishExpr := "NULL"
		if req.Approve {
			newStatus = 2
			orderStatus = orderstatus.Refunded
			finishExpr = "NOW()"
		}
		operatorID := adminOperatorID(r)
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer func() { _ = tx.Rollback() }()
		var orderID string
		var currentStatus int64
		if err := tx.QueryRowContext(r.Context(), "SELECT order_id, status FROM refund_order WHERE id = ? FOR UPDATE", req.RefundId).Scan(&orderID, &currentStatus); err != nil {
			writeNotFound(w, "refund order not found")
			return
		}
		if currentStatus != 0 && currentStatus != 1 {
			writeConflict(w, "refund order already audited")
			return
		}
		if _, err := tx.ExecContext(r.Context(),
			fmt.Sprintf("UPDATE refund_order SET status = ?, audit_remark = ?, operator_id = ?, audit_time = NOW(), finish_time = %s WHERE id = ?", finishExpr),
			newStatus, strings.TrimSpace(req.Remark), operatorID, req.RefundId); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if req.Approve {
			result, err := tx.ExecContext(r.Context(), "UPDATE orders SET status = ?, refunded_at = NOW() WHERE id = ? AND status = ?", orderStatus, orderID, orderstatus.RefundRequested)
			if err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			if rows, err := result.RowsAffected(); err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			} else if rows == 0 {
				writeConflict(w, "order status changed concurrently")
				return
			}
			if _, err := tx.ExecContext(r.Context(),
				"INSERT INTO order_status_log (order_id, from_status, to_status, operator_id, remark) VALUES (?, ?, ?, ?, ?)",
				orderID, orderstatus.RefundRequested, orderstatus.Refunded, operatorID, "refund approved: "+req.Remark); err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
		} else {
			restoreStatus := orderstatus.Paid
			err := tx.QueryRowContext(r.Context(),
				`SELECT from_status
				   FROM order_status_log
				  WHERE order_id = ? AND to_status = ?
				  ORDER BY id DESC LIMIT 1`,
				orderID, orderstatus.RefundRequested).Scan(&restoreStatus)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			if restoreStatus != orderstatus.Paid && restoreStatus != orderstatus.Shipped {
				restoreStatus = orderstatus.Paid
			}
			result, err := tx.ExecContext(r.Context(), "UPDATE orders SET status = ?, refund_requested_at = NULL WHERE id = ? AND status = ?", restoreStatus, orderID, orderstatus.RefundRequested)
			if err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			if rows, err := result.RowsAffected(); err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			} else if rows == 0 {
				writeConflict(w, "order status changed concurrently")
				return
			}
			if _, err := tx.ExecContext(r.Context(),
				"INSERT INTO order_status_log (order_id, from_status, to_status, operator_id, remark) VALUES (?, ?, ?, ?, ?)",
				orderID, orderstatus.RefundRequested, restoreStatus, operatorID, "refund rejected: "+req.Remark); err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
		}
		eventType := "refund.rejected"
		if req.Approve {
			eventType = "refund.succeeded"
		}
		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO order_outbox (event_id, event_type, aggregate_id, payload, status)
			 VALUES (?, ?, ?, JSON_OBJECT('refund_id', ?, 'order_id', ?, 'operator_id', ?), 0)
			 ON DUPLICATE KEY UPDATE status = 0, next_retry_at = NOW(), last_error = ''`,
			"evt_"+req.RefundId+"_"+eventType, eventType, orderID, req.RefundId, orderID, operatorID); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err := tx.Commit(); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, types.AdminRefundAuditResp{RefundId: req.RefundId, Status: refundStatusText(newStatus)})
	}
}

func AdminReconciliationListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminReconciliationReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if req.Page <= 0 {
			req.Page = 1
		}
		if req.PageSize <= 0 || req.PageSize > 100 {
			req.PageSize = 20
		}
		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		where, args := "1=1", []any{}
		if req.Status >= 0 {
			where += " AND status = ?"
			args = append(args, req.Status)
		}
		if req.IssueType != "" {
			where += " AND issue_type = ?"
			args = append(args, strings.TrimSpace(req.IssueType))
		}
		if req.OrderId != "" {
			where += " AND order_id = ?"
			args = append(args, strings.TrimSpace(req.OrderId))
		}
		var total int64
		if err := db.QueryRowContext(r.Context(), fmt.Sprintf("SELECT COUNT(*) FROM reconciliation_issue WHERE %s", where), args...).Scan(&total); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		args = append(args, req.PageSize, (req.Page-1)*req.PageSize)
		rows, err := db.QueryContext(r.Context(), fmt.Sprintf(
			`SELECT id, issue_type, order_id, payment_order_id, refund_order_id, expected_amount_fen,
			        actual_amount_fen, severity, status, detail, COALESCE(create_time,'')
			   FROM reconciliation_issue WHERE %s ORDER BY id DESC LIMIT ? OFFSET ?`, where), args...)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer func() { _ = rows.Close() }()
		items := make([]types.AdminReconciliationItem, 0)
		for rows.Next() {
			var item types.AdminReconciliationItem
			if err := rows.Scan(&item.Id, &item.IssueType, &item.OrderId, &item.PaymentOrderId, &item.RefundOrderId,
				&item.ExpectedAmountFen, &item.ActualAmountFen, &item.Severity, &item.Status, &item.Detail, &item.CreateTime); err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			items = append(items, item)
		}
		httpx.OkJsonCtx(r.Context(), w, types.AdminReconciliationResp{Items: items, Total: total})
	}
}

func AdminReconciliationScanHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		inserted, err := scanReconciliationIssues(r, db)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, map[string]any{"inserted": inserted})
	}
}

func AdminEventListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminEventListReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if req.Page <= 0 {
			req.Page = 1
		}
		if req.PageSize <= 0 || req.PageSize > 100 {
			req.PageSize = 20
		}
		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		where, args := "1=1", []any{}
		if req.Status >= 0 {
			where += " AND status = ?"
			args = append(args, req.Status)
		}
		if req.EventType != "" {
			where += " AND event_type = ?"
			args = append(args, strings.TrimSpace(req.EventType))
		}
		if req.AggregateId != "" {
			where += " AND aggregate_id = ?"
			args = append(args, strings.TrimSpace(req.AggregateId))
		}
		var total int64
		if err := db.QueryRowContext(r.Context(), fmt.Sprintf("SELECT COUNT(*) FROM order_outbox WHERE %s", where), args...).Scan(&total); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		args = append(args, req.PageSize, (req.Page-1)*req.PageSize)
		rows, err := db.QueryContext(r.Context(), fmt.Sprintf(
			`SELECT id, event_id, event_type, aggregate_id, status, attempt_count, last_error,
			        COALESCE(create_time,''), COALESCE(update_time,'')
			   FROM order_outbox WHERE %s ORDER BY id DESC LIMIT ? OFFSET ?`, where), args...)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer func() { _ = rows.Close() }()
		items := make([]types.AdminEventItem, 0)
		for rows.Next() {
			var item types.AdminEventItem
			if err := rows.Scan(&item.Id, &item.EventId, &item.EventType, &item.AggregateId, &item.Status,
				&item.AttemptCount, &item.LastError, &item.CreateTime, &item.UpdateTime); err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			items = append(items, item)
		}
		httpx.OkJsonCtx(r.Context(), w, types.AdminEventListResp{Items: items, Total: total})
	}
}

func AdminEventRetryHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminEventRetryReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.EventId = strings.TrimSpace(req.EventId)
		if req.EventId == "" {
			writeBadRequest(w, "event_id required")
			return
		}
		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if _, err := db.ExecContext(r.Context(),
			"UPDATE order_outbox SET status = 0, next_retry_at = NOW(), last_error = '' WHERE event_id = ?",
			req.EventId); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, types.AdminEventRetryResp{EventId: req.EventId, Status: "pending"})
	}
}

func scanReconciliationIssues(r *http.Request, db *sql.DB) (int64, error) {
	now := time.Now().Format("20060102150405")
	result, err := db.ExecContext(r.Context(),
		`INSERT INTO reconciliation_issue (issue_type, order_id, payment_order_id, expected_amount_fen, actual_amount_fen, severity, detail)
		 SELECT 'payment_amount_mismatch', o.id, p.id, COALESCE(s.payable_amount_fen,0), COALESCE(p.payable_amount_fen,0), 3,
		        CONCAT('scan:', ?)
		   FROM orders o
		   LEFT JOIN order_price_snapshot s ON s.order_id = o.id
		   JOIN payment_order p ON p.order_id = o.id
		  WHERE p.status = 1 AND COALESCE(s.payable_amount_fen,0) <> COALESCE(p.payable_amount_fen,0)
		    AND NOT EXISTS (
		      SELECT 1 FROM reconciliation_issue i
		       WHERE i.issue_type = 'payment_amount_mismatch' AND i.order_id = o.id AND i.status = 0
		    )`, now)
	if err != nil {
		logx.WithContext(r.Context()).Errorf("reconciliation scan failed: %v", err)
		return 0, err
	}
	return result.RowsAffected()
}

func refundStatusText(status int64) string {
	switch status {
	case 0:
		return "requested"
	case 1:
		return "approved"
	case 2:
		return "success"
	case 3:
		return "rejected"
	case 4:
		return "failed"
	default:
		return "unknown"
	}
}
