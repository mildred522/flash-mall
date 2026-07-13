package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	orderpb "flash-mall/app/order/rpc/order"
)

const (
	refundStatusRequested int64 = iota
	refundStatusApproved
	refundStatusSuccess
	refundStatusRejected
	refundStatusFailed
)

func refundIDForOrder(orderID string) string {
	return "rf:" + orderID
}

func insertRefundStatusLog(ctx context.Context, tx *sql.Tx, orderID string, fromStatus, toStatus, operatorID int64, remark string) error {
	_, err := tx.ExecContext(ctx,
		"INSERT INTO order_status_log (order_id, from_status, to_status, operator_id, remark) VALUES (?, ?, ?, ?, ?)",
		orderID, fromStatus, toStatus, operatorID, remark,
	)
	return err
}

func insertRefundOutbox(ctx context.Context, tx *sql.Tx, eventType, refundID, orderID string, payload map[string]any) error {
	eventID := fmt.Sprintf("%s:%s", eventType, refundID)
	payload["event_id"] = eventID
	payload["event_type"] = eventType
	payload["refund_id"] = refundID
	payload["order_id"] = orderID
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO order_outbox (event_id, event_type, aggregate_id, payload, status, next_retry_at)
VALUES (?, ?, ?, CAST(? AS JSON), 0, NOW())
ON DUPLICATE KEY UPDATE status=0, next_retry_at=NOW(), last_error=''`,
		eventID, eventType, orderID, string(body),
	)
	return err
}

func requestRefundResponse(refundID, orderID string, orderStatus, refundStatus int64, repeated bool) *orderpb.RequestRefundResp {
	return &orderpb.RequestRefundResp{
		RefundId: refundID, OrderId: orderID, OrderStatus: orderStatus,
		RefundStatus: refundStatus, Repeated: repeated,
	}
}

func auditRefundResponse(refundID, orderID string, orderStatus, refundStatus int64, repeated bool) *orderpb.AuditRefundResp {
	return &orderpb.AuditRefundResp{
		RefundId: refundID, OrderId: orderID, OrderStatus: orderStatus,
		RefundStatus: refundStatus, Repeated: repeated,
	}
}

func trimRefundError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if len(message) > 255 {
		return message[:255]
	}
	return message
}
