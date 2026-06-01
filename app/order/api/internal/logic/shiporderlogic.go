package logic

import (
	"context"

	"flash-mall/app/order/api/internal/metrics"
	"flash-mall/app/order/api/internal/svc"
	"flash-mall/app/order/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ShipOrderLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewShipOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ShipOrderLogic {
	return &ShipOrderLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ShipOrderLogic) ShipOrder(req *types.ShipOrderReq, userID int64) (*types.ShipOrderResp, error) {
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
	if currentStatus != 1 {
		return nil, status.Error(codes.FailedPrecondition, "order is not in paid status")
	}

	// Update status: paid(1) -> shipped(3)
	result, err := db.ExecContext(l.ctx,
		"UPDATE orders SET status = 3, shipped_at = NOW() WHERE id = ? AND status = 1", req.OrderId,
	)
	if err != nil {
		return nil, status.Error(codes.Internal, "update order status failed")
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, status.Error(codes.FailedPrecondition, "order status changed concurrently")
	}

	// Insert status log
	_, _ = db.ExecContext(l.ctx,
		"INSERT INTO order_status_log (order_id, from_status, to_status, operator_id, remark) VALUES (?, 1, 3, ?, 'seller shipped')",
		req.OrderId, userID,
	)

	l.Infof("order shipped: order_id=%s user_id=%d", req.OrderId, userID)
	metrics.OrderStatusTransitionTotal.WithLabelValues("1", "3").Inc()

	return &types.ShipOrderResp{
		OrderId: req.OrderId,
		Status:  "shipped",
	}, nil
}
