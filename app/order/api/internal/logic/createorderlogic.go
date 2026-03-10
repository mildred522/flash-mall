package logic

import (
	"context"
	"strings"
	"time"

	"flash-mall/app/order/api/internal/metrics"
	"flash-mall/app/order/api/internal/model"
	"flash-mall/app/order/api/internal/svc"
	"flash-mall/app/order/api/internal/types"
	"flash-mall/app/order/rpc/order"
	"flash-mall/app/product/rpc/product"

	"github.com/dtm-labs/dtm/client/dtmgrpc"
	red "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	redisx "github.com/zeromicro/go-zero/core/stores/redis"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	OrderDelayQueueKey           = "order:delay:queue"
	OrderRequestLockKeyPrefix    = "order:request:lock:"
	OrderRequestResultKeyPrefix  = "order:request:result:"
	OrderRequestReverseKeyPrefix = "order:request:order:"
)

type CreateOrderLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateOrderLogic {
	return &CreateOrderLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateOrderLogic) CreateOrder(req *types.CreateOrderReq) (resp *types.CreateOrderResp, err error) {
	requestId := strings.TrimSpace(req.RequestId)
	l.Infow("create order request",
		logx.Field("request_id", requestId),
		logx.Field("user_id", req.UserId),
		logx.Field("product_id", req.ProductId),
		logx.Field("amount", req.Amount),
	)

	result := "fail"
	defer func() {
		metrics.OrderCreateTotal.WithLabelValues(result).Inc()
	}()

	requestTTLSeconds := l.svcCtx.Config.RequestIdTTLSeconds
	if requestTTLSeconds <= 0 {
		requestTTLSeconds = 24 * 60 * 60
	}

	if requestId != "" {
		requestLockKey := OrderRequestLockKeyPrefix + requestId
		requestResultKey := OrderRequestResultKeyPrefix + requestId

		if existOrder, lookupErr := l.svcCtx.OrderModel.FindOneByRequestId(l.ctx, requestId); lookupErr == nil {
			_ = l.svcCtx.Redis.SetexCtx(l.ctx, requestResultKey, existOrder.Id, int(requestTTLSeconds))
			_, _ = l.svcCtx.Redis.DelCtx(l.ctx, requestLockKey)

			l.Infow("request_id hit db cache",
				logx.Field("request_id", requestId),
				logx.Field("order_id", existOrder.Id),
			)
			result = "hit"
			return &types.CreateOrderResp{OrderId: existOrder.Id}, nil
		} else if lookupErr != model.ErrNotFound {
			return nil, lookupErr
		}

		if cachedOrderId, _ := l.svcCtx.Redis.GetCtx(l.ctx, requestResultKey); cachedOrderId != "" {
			l.Infow("request_id hit redis cache",
				logx.Field("request_id", requestId),
				logx.Field("order_id", cachedOrderId),
			)
			result = "hit"
			return &types.CreateOrderResp{OrderId: cachedOrderId}, nil
		}
	}

	if l.svcCtx.Config.DtmServer == "" {
		return nil, status.Error(codes.Internal, "DTM server not configured")
	}

	var orderID string
	if requestId == "" {
		orderID = l.svcCtx.OrderIdGen.NextID()
		requestId = orderID
	}

	requestLockKey := OrderRequestLockKeyPrefix + requestId
	requestResultKey := OrderRequestResultKeyPrefix + requestId
	setOk, setErr := l.svcCtx.Redis.SetnxExCtx(l.ctx, requestLockKey, "processing", int(requestTTLSeconds))
	if setErr != nil {
		return nil, status.Error(codes.Internal, "request_id lock failed")
	}
	if !setOk {
		if cachedOrderId, _ := l.svcCtx.Redis.GetCtx(l.ctx, requestResultKey); cachedOrderId != "" {
			l.Infow("request_id locked, return existing",
				logx.Field("request_id", requestId),
				logx.Field("order_id", cachedOrderId),
			)
			result = "hit"
			return &types.CreateOrderResp{OrderId: cachedOrderId}, nil
		}
		return nil, status.Error(codes.Aborted, "duplicate request in progress")
	}

	if orderID == "" {
		orderID = l.svcCtx.OrderIdGen.NextID()
	}
	gid := dtmgrpc.MustGenGid(l.svcCtx.Config.DtmServer)

	saga := dtmgrpc.NewSagaGrpc(l.svcCtx.Config.DtmServer, gid)
	if l.svcCtx.Config.DtmTimeoutToFailSeconds > 0 {
		saga.TimeoutToFail = l.svcCtx.Config.DtmTimeoutToFailSeconds
	}
	if l.svcCtx.Config.DtmRequestTimeoutSeconds > 0 {
		saga.WithGlobalTransRequestTimeout(l.svcCtx.Config.DtmRequestTimeoutSeconds)
	}
	if l.svcCtx.Config.DtmWaitResult {
		saga.WaitResult = true
	}

	if l.svcCtx.Config.OrderRpcTarget == "" {
		return nil, status.Error(codes.Internal, "Order RPC target not configured")
	}
	if l.svcCtx.Config.ProductRpcTarget == "" {
		return nil, status.Error(codes.Internal, "Product RPC target not configured")
	}

	orderRoute := l.svcCtx.Config.OrderRpcTarget + "/order.Order"
	productRoute := l.svcCtx.Config.ProductRpcTarget + "/product.Product"

	preDeductReq := &order.PreDeductReq{
		ProductId: req.ProductId,
		Amount:    req.Amount,
		OrderId:   orderID,
	}
	createOrderReq := &order.CreateOrderReq{
		OrderId:   orderID,
		RequestId: requestId,
		UserId:    req.UserId,
		ProductId: req.ProductId,
		Amount:    req.Amount,
	}
	deductReq := &product.DeductReq{
		Id:      req.ProductId,
		Num:     req.Amount,
		OrderId: orderID,
	}

	saga.Add(orderRoute+"/PreDeduct", orderRoute+"/PreDeductRollback", preDeductReq)
	saga.Add(orderRoute+"/CreateOrder", orderRoute+"/CreateOrderRollback", createOrderReq)
	saga.Add(productRoute+"/Deduct", productRoute+"/DeductRollback", deductReq)

	if err = saga.Submit(); err != nil {
		l.Errorf("submit SAGA failed: %v", err)
		metrics.OrderSagaSubmitTotal.WithLabelValues("fail").Inc()
		_, _ = l.svcCtx.Redis.DelCtx(l.ctx, requestLockKey)
		return nil, status.Error(codes.Unavailable, "order system busy")
	}
	metrics.OrderSagaSubmitTotal.WithLabelValues("success").Inc()

	if setResultErr := l.svcCtx.Redis.SetexCtx(l.ctx, requestResultKey, orderID, int(requestTTLSeconds)); setResultErr != nil {
		l.Errorf("set request result key failed: %v, request_id=%s, order_id=%s", setResultErr, requestId, orderID)
	} else {
		_, _ = l.svcCtx.Redis.DelCtx(l.ctx, requestLockKey)
	}

	l.Infof("SAGA submitted, gid=%s order_id=%s", gid, orderID)

	delaySeconds := l.svcCtx.Config.OrderTimeoutSeconds
	if delaySeconds <= 0 {
		delaySeconds = 60
	}
	executionTime := time.Now().Add(time.Second * time.Duration(delaySeconds)).Unix()

	var zaddCmd *red.IntCmd
	var setCmd *red.StatusCmd
	pipeErr := l.svcCtx.Redis.PipelinedCtx(l.ctx, func(pipe redisx.Pipeliner) error {
		zaddCmd = pipe.ZAdd(l.ctx, OrderDelayQueueKey, red.Z{Score: float64(executionTime), Member: orderID})
		setCmd = pipe.SetEx(l.ctx, OrderRequestReverseKeyPrefix+orderID, requestId, time.Duration(requestTTLSeconds)*time.Second)
		return nil
	})
	if pipeErr != nil {
		l.Errorf("delay queue pipeline failed: %v, order_id=%s", pipeErr, orderID)
	} else {
		if _, zerr := zaddCmd.Result(); zerr != nil {
			l.Errorf("add delay queue failed: %v, order_id=%s", zerr, orderID)
		}
		if setCmd != nil {
			if _, serr := setCmd.Result(); serr != nil {
				l.Errorf("set request reverse mapping failed: %v, order_id=%s", serr, orderID)
			}
		}
	}

	result = "success"
	return &types.CreateOrderResp{OrderId: orderID}, nil
}
