package logic

import (
	"context"

	"flash-mall/app/common/orderstatus"
	"flash-mall/app/common/paymentstatus"
	"flash-mall/app/entry/api/internal/metrics"
	"flash-mall/app/entry/api/internal/svc"
	"flash-mall/app/entry/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PayOrderLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPayOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PayOrderLogic {
	return &PayOrderLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PayOrderLogic) PayOrder(req *types.PayOrderReq, userID int64) (*types.PayOrderResp, error) {
	metricResult := "fail"
	defer func() { metrics.PaymentTotal.WithLabelValues(metricResult).Inc() }()

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
	err = db.QueryRowContext(l.ctx,
		"SELECT status, user_id FROM orders WHERE id = ? LIMIT 1", req.OrderId,
	).Scan(&currentStatus, &orderUserID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "order not found")
	}
	if orderUserID != userID {
		return nil, status.Error(codes.PermissionDenied, "order does not belong to user")
	}
	if currentStatus != orderstatus.PendingPayment {
		return nil, status.Error(codes.FailedPrecondition, "order is not pending payment")
	}

	// Update order and payment status in a transaction
	tx, err := db.BeginTx(l.ctx, nil)
	if err != nil {
		return nil, status.Error(codes.Internal, "begin tx failed")
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(l.ctx,
		"UPDATE orders SET status = ? WHERE id = ? AND status = ?",
		orderstatus.Paid, req.OrderId, orderstatus.PendingPayment,
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

	_, err = tx.ExecContext(l.ctx,
		"UPDATE payment_order SET status = ? WHERE order_id = ?",
		paymentstatus.Success, req.OrderId,
	)
	if err != nil {
		return nil, status.Error(codes.Internal, "update payment status failed")
	}
	if _, err = tx.ExecContext(l.ctx,
		"INSERT INTO order_status_log (order_id, from_status, to_status, operator_id, remark) VALUES (?, ?, ?, ?, 'buyer paid')",
		req.OrderId, orderstatus.PendingPayment, orderstatus.Paid, userID,
	); err != nil {
		return nil, status.Error(codes.Internal, "insert order status log failed")
	}

	if err = tx.Commit(); err != nil {
		return nil, status.Error(codes.Internal, "commit failed")
	}
	if l.svcCtx.InventoryClient != nil {
		if err := l.svcCtx.InventoryClient.ConfirmDeduct(l.ctx, req.OrderId); err != nil {
			l.Errorf("inventory confirm deduct failed: order_id=%s err=%v", req.OrderId, err)
		}
	}

	// Remove from delay queue so CloseOrderJob won't close it
	if _, err := l.svcCtx.Redis.ZremCtx(l.ctx, OrderDelayQueueKey, req.OrderId); err != nil {
		l.Errorf("remove paid order from delay queue failed: order_id=%s err=%v", req.OrderId, err)
	}

	l.Infof("order paid: order_id=%s user_id=%d", req.OrderId, userID)
	metricResult = "ok"
	metrics.OrderStatusTransitionTotal.WithLabelValues("0", "1").Inc()

	return &types.PayOrderResp{
		OrderId: req.OrderId,
		Status:  "paid",
	}, nil
}
