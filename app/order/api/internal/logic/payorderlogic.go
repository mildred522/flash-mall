package logic

import (
	"context"
	"encoding/json"
	"fmt"

	"flash-mall/app/order/api/internal/metrics"
	"flash-mall/app/order/api/internal/svc"
	"flash-mall/app/order/api/internal/types"
	"flash-mall/app/order/rpc/orderclient"

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

	var currentStatus int64
	var orderUserID int64
	var paymentOrderID string
	var outTradeNo string
	var payableAmountFen int64
	err = db.QueryRowContext(l.ctx,
		`SELECT o.status, o.user_id, p.id, p.out_trade_no, p.payable_amount_fen
FROM orders o
JOIN payment_order p ON p.order_id = o.id
WHERE o.id = ? LIMIT 1`, req.OrderId,
	).Scan(&currentStatus, &orderUserID, &paymentOrderID, &outTradeNo, &payableAmountFen)
	if err != nil {
		return nil, status.Error(codes.NotFound, "order not found")
	}
	if orderUserID != userID {
		return nil, status.Error(codes.PermissionDenied, "order does not belong to user")
	}
	if currentStatus != 0 {
		return nil, status.Error(codes.FailedPrecondition, "order is not pending payment")
	}
	if l.svcCtx.OrderRpc == nil {
		return nil, status.Error(codes.Internal, "order rpc not configured")
	}

	callbackBody, err := mockPaymentCallbackBody(paymentOrderID, outTradeNo, payableAmountFen)
	if err != nil {
		return nil, status.Error(codes.Internal, "build payment callback failed")
	}
	if _, err := l.svcCtx.OrderRpc.MarkOrderPaid(l.ctx, &orderclient.MarkOrderPaidReq{
		OrderId:        req.OrderId,
		PaymentOrderId: paymentOrderID,
		OutTradeNo:     outTradeNo,
		CallbackBody:   callbackBody,
	}); err != nil {
		return nil, err
	}

	if l.svcCtx.Redis != nil {
		_, _ = l.svcCtx.Redis.ZremCtx(l.ctx, OrderDelayQueueKey, req.OrderId)
	}

	l.Infof("order paid: order_id=%s user_id=%d", req.OrderId, userID)
	metricResult = "ok"
	metrics.OrderStatusTransitionTotal.WithLabelValues("0", "1").Inc()

	return &types.PayOrderResp{
		OrderId: req.OrderId,
		Status:  "paid",
	}, nil
}

func mockPaymentCallbackBody(paymentOrderID, outTradeNo string, paidAmountFen int64) (string, error) {
	body, err := json.Marshal(map[string]any{
		"trade_status":    "SUCCESS",
		"source":          "mock",
		"provider":        "mock",
		"event_id":        fmt.Sprintf("mock:%s:%s", paymentOrderID, outTradeNo),
		"paid_amount_fen": paidAmountFen,
	})
	if err != nil {
		return "", err
	}
	return string(body), nil
}
