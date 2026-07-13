package handler

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/orderstatus"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func AdminRefundListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		req, err := adminRefundQueryFromRequest(c)
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, err)
			return
		}
		db, err := orderDB(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order datasource unavailable", err))
			return
		}
		resp, err := loadAdminRefunds(ctx, db, req)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin refund query failed", err))
			return
		}
		ok(ctx, c, resp)
	}
}

func AdminRefundAuditHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req AdminRefundAuditReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid refund audit request"))
			return
		}
		req.RefundID = strings.TrimSpace(req.RefundID)
		req.Remark = strings.TrimSpace(req.Remark)
		if req.RefundID == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "refund_id is required"))
			return
		}
		db, err := orderDB(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order datasource unavailable", err))
			return
		}
		operatorID := gatewayOperatorID(ctx)
		statusText, err := auditAdminRefund(ctx, svcCtx, db, req, operatorID)
		if err != nil {
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		ok(ctx, c, AdminRefundAuditResp{RefundID: req.RefundID, Status: statusText})
	}
}

func adminRefundQueryFromRequest(c *app.RequestContext) (AdminRefundListReq, error) {
	page, err := parseInt64Default(c.Query("page"), 1)
	if err != nil {
		return AdminRefundListReq{}, apperror.New(apperror.CodeInvalidArgument, "page must be numeric")
	}
	pageSize, err := parseInt64Default(c.Query("page_size"), 20)
	if err != nil {
		return AdminRefundListReq{}, apperror.New(apperror.CodeInvalidArgument, "page_size must be numeric")
	}
	status, err := parseInt64Default(c.Query("status"), -1)
	if err != nil {
		return AdminRefundListReq{}, apperror.New(apperror.CodeInvalidArgument, "status must be numeric")
	}
	merchantID, err := parseOptionalInt64(c.Query("merchant_id"))
	if err != nil {
		return AdminRefundListReq{}, apperror.New(apperror.CodeInvalidArgument, "merchant_id must be numeric")
	}
	userID, err := parseOptionalInt64(c.Query("user_id"))
	if err != nil {
		return AdminRefundListReq{}, apperror.New(apperror.CodeInvalidArgument, "user_id must be numeric")
	}
	return AdminRefundListReq{
		Page:       normalizePage(page),
		PageSize:   normalizeAdminPageSize(pageSize),
		MerchantID: merchantID,
		Status:     status,
		UserID:     userID,
		OrderID:    strings.TrimSpace(c.Query("order_id")),
	}, nil
}

func loadAdminRefunds(ctx context.Context, db *sql.DB, req AdminRefundListReq) (AdminRefundListResp, error) {
	where := "1=1"
	args := []any{}
	if req.Status >= 0 {
		where += " AND r.status = ?"
		args = append(args, req.Status)
	}
	if req.UserID > 0 {
		where += " AND r.user_id = ?"
		args = append(args, req.UserID)
	}
	if req.MerchantID > 0 {
		where += " AND r.merchant_id = ?"
		args = append(args, req.MerchantID)
	}
	if req.OrderID != "" {
		where += " AND r.order_id = ?"
		args = append(args, req.OrderID)
	}
	var total int64
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM refund_order r WHERE "+where, args...).Scan(&total); err != nil {
		return AdminRefundListResp{}, err
	}
	queryArgs := append(append([]any{}, args...), req.PageSize, (req.Page-1)*req.PageSize)
	rows, err := db.QueryContext(ctx, `SELECT r.id,
       r.order_id,
       r.payment_order_id,
       r.user_id,
       r.merchant_id,
       COALESCE(m.name, ''),
       r.product_id,
       r.refund_amount_fen,
       r.status,
       r.reason,
       r.audit_remark,
       r.operator_id,
       DATE_FORMAT(r.request_time, '%Y-%m-%d %H:%i:%s'),
       COALESCE(DATE_FORMAT(r.audit_time, '%Y-%m-%d %H:%i:%s'), ''),
       COALESCE(DATE_FORMAT(r.finish_time, '%Y-%m-%d %H:%i:%s'), '')
FROM refund_order r
LEFT JOIN merchant m ON m.id = r.merchant_id
WHERE `+where+`
ORDER BY r.create_time DESC
LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return AdminRefundListResp{}, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]AdminRefundItem, 0)
	for rows.Next() {
		var item AdminRefundItem
		if err := rows.Scan(&item.RefundID, &item.OrderID, &item.PaymentOrderID, &item.UserID, &item.MerchantID, &item.MerchantName, &item.ProductID, &item.RefundAmountFen, &item.Status, &item.Reason, &item.AuditRemark, &item.OperatorID, &item.RequestTime, &item.AuditTime, &item.FinishTime); err != nil {
			return AdminRefundListResp{}, err
		}
		item.StatusText = refundStatusText(item.Status)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return AdminRefundListResp{}, err
	}
	return AdminRefundListResp{Items: items, Total: total}, nil
}

func auditAdminRefund(ctx context.Context, svcCtx *svc.ServiceContext, db *sql.DB, req AdminRefundAuditReq, operatorID int64) (string, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()

	var orderID string
	var refundStatus int64
	err = tx.QueryRowContext(ctx, `
SELECT r.order_id, r.status
FROM refund_order r
JOIN orders o ON o.id = r.order_id
WHERE r.id = ?
FOR UPDATE`, req.RefundID).Scan(&orderID, &refundStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", apperror.New(apperror.CodeRefundNotFound, "refund order not found")
		}
		return "", err
	}
	if refundStatus != 0 && refundStatus != 1 {
		return "", apperror.New(apperror.CodeRefundStatusInvalid, "refund order already audited")
	}

	newRefundStatus := int64(3)
	finishExpr := "NULL"
	if req.Approve {
		newRefundStatus = 2
		finishExpr = "NOW()"
	}
	if _, err = tx.ExecContext(ctx,
		"UPDATE refund_order SET status = ?, audit_remark = ?, operator_id = ?, audit_time = NOW(), finish_time = "+finishExpr+" WHERE id = ?",
		newRefundStatus, req.Remark, operatorID, req.RefundID,
	); err != nil {
		return "", err
	}

	if req.Approve {
		result, err := tx.ExecContext(ctx,
			"UPDATE orders SET status = ?, refunded_at = NOW() WHERE id = ? AND status = ?",
			orderstatus.Refunded, orderID, orderstatus.RefundRequested,
		)
		if err != nil {
			return "", err
		}
		if rows, err := result.RowsAffected(); err != nil {
			return "", err
		} else if rows == 0 {
			return "", apperror.New(apperror.CodeOrderStatusInvalid, "order status changed concurrently")
		}
		if _, err = tx.ExecContext(ctx,
			"INSERT INTO order_status_log (order_id, from_status, to_status, operator_id, remark) VALUES (?, ?, ?, ?, ?)",
			orderID, orderstatus.RefundRequested, orderstatus.Refunded, operatorID, "refund approved: "+req.Remark,
		); err != nil {
			return "", err
		}
		if err = releaseInventoryStock(ctx, svcCtx, InventoryReleaseReq{OrderID: orderID, Reason: "refund approved"}); err != nil {
			return "", apperror.Wrap(apperror.CodeStockReconcileFailed, "release order stock failed", err)
		}
	} else {
		restoreStatus, err := restoreStatusBeforeRefund(ctx, tx, orderID)
		if err != nil {
			return "", err
		}
		result, err := tx.ExecContext(ctx,
			"UPDATE orders SET status = ?, refund_requested_at = NULL WHERE id = ? AND status = ?",
			restoreStatus, orderID, orderstatus.RefundRequested,
		)
		if err != nil {
			return "", err
		}
		if rows, err := result.RowsAffected(); err != nil {
			return "", err
		} else if rows == 0 {
			return "", apperror.New(apperror.CodeOrderStatusInvalid, "order status changed concurrently")
		}
		if _, err = tx.ExecContext(ctx,
			"INSERT INTO order_status_log (order_id, from_status, to_status, operator_id, remark) VALUES (?, ?, ?, ?, ?)",
			orderID, orderstatus.RefundRequested, restoreStatus, operatorID, "refund rejected: "+req.Remark,
		); err != nil {
			return "", err
		}
	}

	eventType := "refund.rejected"
	if req.Approve {
		eventType = "refund.succeeded"
	}
	if _, err = tx.ExecContext(ctx,
		`INSERT INTO order_outbox (event_id, event_type, aggregate_id, payload, status)
		 VALUES (?, ?, ?, JSON_OBJECT('refund_id', ?, 'order_id', ?, 'operator_id', ?), 0)
		 ON DUPLICATE KEY UPDATE status = 0, next_retry_at = NOW(), last_error = ''`,
		"evt_"+req.RefundID+"_"+eventType, eventType, orderID, req.RefundID, orderID, operatorID,
	); err != nil {
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	return refundStatusText(newRefundStatus), nil
}

func restoreStatusBeforeRefund(ctx context.Context, tx *sql.Tx, orderID string) (int64, error) {
	restoreStatus := orderstatus.Paid
	err := tx.QueryRowContext(ctx, `
SELECT from_status
FROM order_status_log
WHERE order_id = ? AND to_status = ?
ORDER BY id DESC
LIMIT 1`, orderID, orderstatus.RefundRequested).Scan(&restoreStatus)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	if restoreStatus != orderstatus.Paid && restoreStatus != orderstatus.Shipped {
		return orderstatus.Paid, nil
	}
	return restoreStatus, nil
}
