package logic

import (
	"context"
	"fmt"

	"flash-mall/app/order/api/internal/metrics"
	"flash-mall/app/order/api/internal/svc"
	"flash-mall/app/order/api/internal/types"
	"flash-mall/app/order/rpc/orderclient"
	"flash-mall/app/product/rpc/productclient"

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
	// Can refund if pending(0) or paid(1), but not shipped(3)+ or already closed/refunded
	if currentStatus != 0 && currentStatus != 1 {
		return nil, status.Error(codes.FailedPrecondition, "order cannot be refunded in current status")
	}

	// If pending payment, remove from delay queue first
	if currentStatus == 0 {
		_, _ = l.svcCtx.Redis.ZremCtx(l.ctx, OrderDelayQueueKey, req.OrderId)
	}

	// Update status to refund_requested(5), then immediately to refunded(6) (auto-approve for demo)
	tx, err := db.BeginTx(l.ctx, nil)
	if err != nil {
		return nil, status.Error(codes.Internal, "begin tx failed")
	}
	defer tx.Rollback()

	// Set status to refunded with timestamps
	result, err := tx.ExecContext(l.ctx,
		"UPDATE orders SET status = 6, refund_requested_at = NOW(), refunded_at = NOW() WHERE id = ? AND status = ?",
		req.OrderId, currentStatus,
	)
	if err != nil {
		return nil, status.Error(codes.Internal, "update order status failed")
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, status.Error(codes.FailedPrecondition, "order status changed concurrently")
	}

	// Insert status log for the transition
	_, _ = tx.ExecContext(l.ctx,
		"INSERT INTO order_status_log (order_id, from_status, to_status, operator_id, remark) VALUES (?, ?, 6, ?, ?)",
		req.OrderId, currentStatus, userID, "refund: "+req.Reason,
	)

	if err = tx.Commit(); err != nil {
		return nil, status.Error(codes.Internal, "commit failed")
	}

	// Restore stock (async, best-effort)
	if currentStatus == 1 {
		// Restore DB stock via product RPC
		_, _ = l.svcCtx.ProductRpc.RevertStock(l.ctx, &productclient.RevertStockReq{
			Id:      productID,
			Num:     amount,
			OrderId: req.OrderId,
		})
		// Restore Redis pre-deduct stock
		_, _ = l.svcCtx.OrderRpc.PreDeductRollback(l.ctx, &orderclient.PreDeductReq{
			ProductId: productID,
			Amount:    amount,
			OrderId:   req.OrderId,
		})
	}

	l.Infof("order refunded: order_id=%s user_id=%d from_status=%d", req.OrderId, userID, currentStatus)
	fromStatusStr := fmt.Sprintf("%d", currentStatus)
	metrics.OrderStatusTransitionTotal.WithLabelValues(fromStatusStr, "6").Inc()

	return &types.RefundOrderResp{
		OrderId: req.OrderId,
		Status:  "refunded",
	}, nil
}
