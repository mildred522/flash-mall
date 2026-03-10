package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"flash-mall/app/order/rpc/internal/job"
	"flash-mall/app/order/rpc/internal/svc"
	order "flash-mall/app/order/rpc/order"

	"github.com/dtm-labs/dtm/client/dtmgrpc"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateOrderLogic {
	return &CreateOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateOrder is the order creation saga branch in order-rpc.
func (l *CreateOrderLogic) CreateOrder(in *order.CreateOrderReq) (*order.Empty, error) {
	if in.OrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	requestID := in.RequestId
	if requestID == "" {
		requestID = in.OrderId
	}

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "DTM barrier not found")
	}

	db, err := l.svcCtx.SqlConn.RawDB()
	if err != nil {
		return nil, status.Error(codes.Internal, "db connection failed")
	}

	err = barrier.CallWithDB(db, func(tx *sql.Tx) error {
		if _, execErr := tx.Exec(
			"INSERT IGNORE INTO orders (id, request_id, user_id, product_id, amount, status) VALUES (?, ?, ?, ?, ?, 0)",
			in.OrderId, requestID, in.UserId, in.ProductId, in.Amount,
		); execErr != nil {
			return execErr
		}

		payload, marshalErr := json.Marshal(map[string]any{
			"event_id":   "order.created:" + in.OrderId,
			"event_type": "order.created",
			"order_id":   in.OrderId,
			"request_id": requestID,
			"user_id":    in.UserId,
			"product_id": in.ProductId,
			"amount":     in.Amount,
			"created_at": time.Now().Unix(),
		})
		if marshalErr != nil {
			return marshalErr
		}

		return job.InsertOrderCreatedOutbox(tx, in.OrderId, string(payload))
	})
	if err != nil {
		return nil, err
	}

	return &order.Empty{}, nil
}
