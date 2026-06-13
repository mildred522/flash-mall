package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	order "flash-mall/app/order/rpc/order"
)

const (
	orderStatePendingPayment  int64 = 0
	orderStatePaid            int64 = 1
	orderStateClosed          int64 = 2
	orderStateShipped         int64 = 3
	orderStateCompleted       int64 = 4
	orderStateRefundRequested int64 = 5
	orderStateRefunded        int64 = 6
	orderStateRefundFailed    int64 = 7
)

func lifecycleStatusText(statusCode int64) string {
	switch statusCode {
	case orderStatePendingPayment:
		return "pending_payment"
	case orderStatePaid:
		return "paid"
	case orderStateClosed:
		return "closed"
	case orderStateShipped:
		return "shipped"
	case orderStateCompleted:
		return "completed"
	case orderStateRefundRequested:
		return "refund_requested"
	case orderStateRefunded:
		return "refunded"
	case orderStateRefundFailed:
		return "refund_failed"
	default:
		return "unknown"
	}
}

func lifecycleResp(orderID string, statusCode int64) *order.LifecycleOrderResp {
	return &order.LifecycleOrderResp{
		OrderId:    orderID,
		Status:     statusCode,
		StatusText: lifecycleStatusText(statusCode),
	}
}

func insertOrderStatusLog(ctx context.Context, tx *sql.Tx, orderID string, fromStatus, toStatus, operatorID int64, remark string) error {
	_, err := tx.ExecContext(ctx,
		"INSERT INTO order_status_log (order_id, from_status, to_status, operator_id, remark) VALUES (?, ?, ?, ?, ?)",
		orderID, fromStatus, toStatus, operatorID, remark,
	)
	return err
}

func insertLifecycleOutbox(ctx context.Context, tx *sql.Tx, eventType, orderID string, in *order.LifecycleOrderReq, statusCode int64, errorMessage string) error {
	eventID := fmt.Sprintf("%s:%s", eventType, orderID)
	if in.RequestId != "" {
		eventID = fmt.Sprintf("%s:%s:%s", eventType, orderID, in.RequestId)
	}
	payload, err := json.Marshal(map[string]any{
		"event_id":      eventID,
		"event_type":    eventType,
		"order_id":      orderID,
		"status":        statusCode,
		"status_text":   lifecycleStatusText(statusCode),
		"operator_id":   in.OperatorId,
		"operator_role": in.OperatorRole,
		"reason":        in.Reason,
		"error_message": errorMessage,
		"created_at":    time.Now().Unix(),
	})
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
INSERT IGNORE INTO order_outbox (event_id, event_type, aggregate_id, payload, status, next_retry_at)
VALUES (?, ?, ?, CAST(? AS JSON), 0, NOW())`,
		eventID, eventType, orderID, string(payload),
	)
	return err
}

func trimSQLErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if len(msg) > 255 {
		return msg[:255]
	}
	return msg
}
