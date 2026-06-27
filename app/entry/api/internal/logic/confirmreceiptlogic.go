package logic

import (
	"context"

	"flash-mall/app/common/orderstatus"
	"flash-mall/app/entry/api/internal/metrics"
	"flash-mall/app/entry/api/internal/svc"
	"flash-mall/app/entry/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ConfirmReceiptLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewConfirmReceiptLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfirmReceiptLogic {
	return &ConfirmReceiptLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfirmReceiptLogic) ConfirmReceipt(req *types.ConfirmReceiptReq, userID int64) (*types.ConfirmReceiptResp, error) {
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
	if currentStatus != orderstatus.Shipped {
		return nil, status.Error(codes.FailedPrecondition, "order is not in shipped status")
	}

	tx, err := db.BeginTx(l.ctx, nil)
	if err != nil {
		return nil, status.Error(codes.Internal, "begin tx failed")
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(l.ctx,
		"UPDATE orders SET status = ?, completed_at = NOW() WHERE id = ? AND status = ?",
		orderstatus.Completed, req.OrderId, orderstatus.Shipped,
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

	if _, err = tx.ExecContext(l.ctx,
		"INSERT INTO order_status_log (order_id, from_status, to_status, operator_id, remark) VALUES (?, ?, ?, ?, 'buyer confirmed receipt')",
		req.OrderId, orderstatus.Shipped, orderstatus.Completed, userID,
	); err != nil {
		return nil, status.Error(codes.Internal, "insert order status log failed")
	}
	if err = tx.Commit(); err != nil {
		return nil, status.Error(codes.Internal, "commit failed")
	}

	l.Infof("order completed: order_id=%s user_id=%d", req.OrderId, userID)
	metrics.OrderStatusTransitionTotal.WithLabelValues("3", "4").Inc()

	return &types.ConfirmReceiptResp{
		OrderId: req.OrderId,
		Status:  "completed",
	}, nil
}
