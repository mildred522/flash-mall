package logic

import (
	"context"

	"flash-mall/app/order/api/internal/svc"
	"flash-mall/app/order/api/internal/types"
	"flash-mall/app/order/rpc/orderclient"

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
	err = db.QueryRowContext(l.ctx,
		"SELECT status, user_id FROM orders WHERE id = ? LIMIT 1", req.OrderId,
	).Scan(&currentStatus, &orderUserID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "order not found")
	}
	if orderUserID != userID {
		return nil, status.Error(codes.PermissionDenied, "order does not belong to user")
	}
	if currentStatus != 1 && currentStatus != 5 {
		return nil, status.Error(codes.FailedPrecondition, "order cannot be refunded in current status")
	}
	if l.svcCtx.OrderRpc == nil {
		return nil, status.Error(codes.Internal, "order rpc not configured")
	}

	resp, err := l.svcCtx.OrderRpc.RequestRefund(l.ctx, &orderclient.LifecycleOrderReq{
		OrderId:      req.OrderId,
		OperatorId:   userID,
		OperatorRole: "user",
		Reason:       req.Reason,
		RequestId:    req.OrderId + ":refund-request",
	})
	if err != nil {
		return nil, err
	}

	l.Infof("order refund requested: order_id=%s user_id=%d from_status=%d", req.OrderId, userID, currentStatus)

	return &types.RefundOrderResp{
		OrderId: req.OrderId,
		Status:  resp.StatusText,
	}, nil
}
