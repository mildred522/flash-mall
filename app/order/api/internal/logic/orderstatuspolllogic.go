package logic

import (
	"context"
	"strings"

	"flash-mall/app/order/api/internal/svc"
	"flash-mall/app/order/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OrderStatusPollLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOrderStatusPollLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OrderStatusPollLogic {
	return &OrderStatusPollLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OrderStatusPollLogic) OrderStatusPoll(req *types.OrderStatusPollReq) (*types.OrderStatusPollResp, error) {
	requestId := strings.TrimSpace(req.RequestId)
	if requestId == "" {
		return &types.OrderStatusPollResp{Status: "missing_request_id"}, nil
	}

	resultKey := "order:request:result:" + requestId
	orderId, err := l.svcCtx.Redis.GetCtx(l.ctx, resultKey)
	if err != nil {
		logx.WithContext(l.ctx).Errorf("poll redis get failed: %v", err)
		return &types.OrderStatusPollResp{RequestId: requestId, Status: "error"}, nil
	}

	if orderId != "" {
		return &types.OrderStatusPollResp{RequestId: requestId, OrderId: orderId, Status: "created"}, nil
	}

	lockKey := "order:request:lock:" + requestId
	lockVal, _ := l.svcCtx.Redis.GetCtx(l.ctx, lockKey)
	if lockVal != "" {
		return &types.OrderStatusPollResp{RequestId: requestId, Status: "processing"}, nil
	}

	return &types.OrderStatusPollResp{RequestId: requestId, Status: "not_found"}, nil
}
