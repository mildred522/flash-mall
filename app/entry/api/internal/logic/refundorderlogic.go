package logic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"flash-mall/app/common/orderstatus"
	"flash-mall/app/entry/api/internal/metrics"
	"flash-mall/app/entry/api/internal/svc"
	"flash-mall/app/entry/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RefundOrderLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRefundOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefundOrderLogic {
	return &RefundOrderLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RefundOrderLogic) RefundOrder(req *types.RefundOrderReq, userID int64) (*types.RefundOrderResp, error) {
	if req.OrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	db, err := l.svcCtx.SqlConn.RawDB()
	if err != nil {
		return nil, status.Error(codes.Internal, "db connection failed")
	}

	// Verify order exists and belongs to user
	var currentStatus int64
	var orderUserID int64
	var productID int64
	var amount int64
	err = db.QueryRowContext(l.ctx,
		"SELECT status, user_id, product_id, amount FROM orders WHERE id = ? LIMIT 1", req.OrderId,
	).Scan(&currentStatus, &orderUserID, &productID, &amount)
	if err != nil {
		return nil, status.Error(codes.NotFound, "order not found")
	}
	if orderUserID != userID {
		return nil, status.Error(codes.PermissionDenied, "order does not belong to user")
	}
	if currentStatus != orderstatus.Paid && currentStatus != orderstatus.Shipped {
		return nil, status.Error(codes.FailedPrecondition, "order cannot be refunded in current status")
	}

	tx, err := db.BeginTx(l.ctx, nil)
	if err != nil {
		return nil, status.Error(codes.Internal, "begin tx failed")
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(l.ctx,
		"UPDATE orders SET status = ?, refund_requested_at = NOW() WHERE id = ? AND status = ?",
		orderstatus.RefundRequested, req.OrderId, currentStatus,
	)
	if err != nil {
		return nil, status.Error(codes.Internal, "update order status failed")
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, status.Error(codes.Internal, "check order update result failed")
	}
	if rows == 0 {
		return nil, status.Error(codes.FailedPrecondition, "order status changed concurrently")
	}

	var paymentOrderID string
	var refundAmountFen int64
	_ = tx.QueryRowContext(l.ctx,
		"SELECT COALESCE(p.id,''), COALESCE(p.payable_amount_fen,0) FROM payment_order p WHERE p.order_id = ? LIMIT 1",
		req.OrderId,
	).Scan(&paymentOrderID, &refundAmountFen)
	refundID := fmt.Sprintf("rf_%s_%d", req.OrderId, time.Now().UnixNano())
	reason := strings.TrimSpace(req.Reason)
	if _, err = tx.ExecContext(l.ctx,
		`INSERT INTO refund_order
		  (id, order_id, payment_order_id, user_id, product_id, refund_amount_fen, status, reason)
		  VALUES (?, ?, ?, ?, ?, ?, 0, ?)`,
		refundID, req.OrderId, paymentOrderID, userID, productID, refundAmountFen, reason); err != nil {
		return nil, status.Error(codes.Internal, "insert refund order failed")
	}
	if _, err = tx.ExecContext(l.ctx,
		"INSERT INTO order_status_log (order_id, from_status, to_status, operator_id, remark) VALUES (?, ?, ?, ?, ?)",
		req.OrderId, currentStatus, orderstatus.RefundRequested, userID, "refund requested: "+reason,
	); err != nil {
		return nil, status.Error(codes.Internal, "insert order status log failed")
	}
	if _, err = tx.ExecContext(l.ctx,
		`INSERT INTO order_outbox (event_id, event_type, aggregate_id, payload, status)
		 VALUES (?, 'refund.requested', ?, JSON_OBJECT('refund_id', ?, 'order_id', ?, 'user_id', ?, 'amount_fen', ?), 0)`,
		"evt_"+refundID, req.OrderId, refundID, req.OrderId, userID, refundAmountFen); err != nil {
		return nil, status.Error(codes.Internal, "insert refund outbox failed")
	}

	if err = tx.Commit(); err != nil {
		return nil, status.Error(codes.Internal, "commit failed")
	}

	l.Infof("refund requested: refund_id=%s order_id=%s user_id=%d from_status=%d amount=%d", refundID, req.OrderId, userID, currentStatus, amount)
	fromStatusStr := fmt.Sprintf("%d", currentStatus)
	metrics.OrderStatusTransitionTotal.WithLabelValues(fromStatusStr, fmt.Sprintf("%d", orderstatus.RefundRequested)).Inc()

	return &types.RefundOrderResp{
		OrderId: req.OrderId,
		Status:  "refund_requested",
	}, nil
}
