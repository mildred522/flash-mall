package logic

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"flash-mall/app/order/rpc/internal/svc"
	order "flash-mall/app/order/rpc/order"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RequestRefundLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRequestRefundLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RequestRefundLogic {
	return &RequestRefundLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RequestRefundLogic) RequestRefund(in *order.LifecycleOrderReq) (*order.LifecycleOrderResp, error) {
	orderID := strings.TrimSpace(in.OrderId)
	if orderID == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	db, err := l.svcCtx.SqlConn.RawDB()
	if err != nil {
		return nil, status.Error(codes.Internal, "db connection failed")
	}

	tx, err := db.BeginTx(l.ctx, nil)
	if err != nil {
		return nil, status.Error(codes.Internal, "begin tx failed")
	}
	defer func() { _ = tx.Rollback() }()

	var currentStatus int64
	err = tx.QueryRowContext(l.ctx, "SELECT status FROM orders WHERE id = ? FOR UPDATE", orderID).Scan(&currentStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "order not found")
		}
		return nil, err
	}

	if currentStatus == orderStateRefundRequested {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return lifecycleResp(orderID, orderStateRefundRequested), nil
	}
	if currentStatus != orderStatePaid {
		return nil, status.Error(codes.FailedPrecondition, "order is not refundable")
	}

	result, err := tx.ExecContext(l.ctx, `
UPDATE orders
SET status = ?, refund_requested_at = NOW(), update_time = NOW()
WHERE id = ? AND status = ?`,
		orderStateRefundRequested, orderID, orderStatePaid,
	)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, status.Error(codes.FailedPrecondition, "order status changed concurrently")
	}

	if err := insertOrderStatusLog(l.ctx, tx, orderID, orderStatePaid, orderStateRefundRequested, in.OperatorId, "refund requested: "+in.Reason); err != nil {
		return nil, err
	}
	if err := insertLifecycleOutbox(l.ctx, tx, "order.refund.requested", orderID, in, orderStateRefundRequested, ""); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return lifecycleResp(orderID, orderStateRefundRequested), nil
}
