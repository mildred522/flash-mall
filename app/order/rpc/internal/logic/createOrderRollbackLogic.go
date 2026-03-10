package logic

import (
	"context"
	"database/sql"

	"flash-mall/app/order/rpc/internal/svc"
	order "flash-mall/app/order/rpc/order"

	"github.com/dtm-labs/dtm/client/dtmgrpc"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateOrderRollbackLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateOrderRollbackLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateOrderRollbackLogic {
	return &CreateOrderRollbackLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateOrderRollback 是订单回滚/关闭的 SAGA 补偿分支。
func (l *CreateOrderRollbackLogic) CreateOrderRollback(in *order.CreateOrderReq) (*order.Empty, error) {
	if in.OrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "DTM barrier not found")
	}

	db, err := l.svcCtx.SqlConn.RawDB()
	if err != nil {
		return nil, status.Error(codes.Internal, "db connection failed")
	}

	// CHG 2026-01-31: 变更=回滚仅更新待支付订单为关闭; 之前=无回滚分支; 原因=确保失败时状态可追溯且幂等。
	err = barrier.CallWithDB(db, func(tx *sql.Tx) error {
		_, execErr := tx.Exec(
			"UPDATE orders SET status = 2 WHERE id = ? AND status = 0",
			in.OrderId,
		)
		return execErr
	})
	if err != nil {
		return nil, err
	}

	return &order.Empty{}, nil
}
