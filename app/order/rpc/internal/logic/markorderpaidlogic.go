package logic

import (
	"context"
	"database/sql"
	"errors"

	"flash-mall/app/order/rpc/internal/svc"
	order "flash-mall/app/order/rpc/order"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type MarkOrderPaidLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMarkOrderPaidLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkOrderPaidLogic {
	return &MarkOrderPaidLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *MarkOrderPaidLogic) MarkPaid(in *order.MarkOrderPaidReq) (*order.MarkOrderPaidResp, error) {
	if in.OrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}
	if in.PaymentOrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "payment_order_id is required")
	}
	if in.OutTradeNo == "" {
		return nil, status.Error(codes.InvalidArgument, "out_trade_no is required")
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

	var orderStatus int64
	if err := tx.QueryRowContext(l.ctx, "SELECT status FROM orders WHERE id = ?", in.OrderId).Scan(&orderStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "order not found")
		}
		return nil, err
	}

	var paymentStatus int64
	if err := tx.QueryRowContext(l.ctx, "SELECT status FROM payment_order WHERE id = ? AND out_trade_no = ?", in.PaymentOrderId, in.OutTradeNo).Scan(&paymentStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "payment order not found")
		}
		return nil, err
	}

	if orderStatus == 1 || paymentStatus == 1 {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &order.MarkOrderPaidResp{Updated: false, OrderStatus: "PAID"}, nil
	}
	if orderStatus == 2 {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &order.MarkOrderPaidResp{Updated: false, OrderStatus: "CLOSED"}, nil
	}

	orderResult, err := tx.ExecContext(l.ctx, "UPDATE orders SET status = 1, update_time = NOW() WHERE id = ? AND status = 0", in.OrderId)
	if err != nil {
		return nil, err
	}
	orderRows, err := orderResult.RowsAffected()
	if err != nil {
		return nil, err
	}
	if orderRows == 0 {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &order.MarkOrderPaidResp{Updated: false, OrderStatus: "UNKNOWN"}, nil
	}

	paymentResult, err := tx.ExecContext(l.ctx, "UPDATE payment_order SET status = 1, paid_at = NOW(), callback_payload = CAST(? AS JSON), update_time = NOW() WHERE id = ? AND out_trade_no = ? AND status = 0", in.CallbackBody, in.PaymentOrderId, in.OutTradeNo)
	if err != nil {
		return nil, err
	}
	paymentRows, err := paymentResult.RowsAffected()
	if err != nil {
		return nil, err
	}
	if paymentRows == 0 {
		return nil, status.Error(codes.FailedPrecondition, "payment order is not payable")
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &order.MarkOrderPaidResp{Updated: true, OrderStatus: "PAID"}, nil
}
