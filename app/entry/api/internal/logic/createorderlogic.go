package logic

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	commonobs "flash-mall/app/common/observability"
	"flash-mall/app/common/orderstatus"
	"flash-mall/app/entry/api/internal/metrics"
	"flash-mall/app/entry/api/internal/model"
	"flash-mall/app/entry/api/internal/svc"
	"flash-mall/app/entry/api/internal/types"
	order "flash-mall/app/order/rpc/order"
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
	ctx                context.Context
	svcCtx             *svc.ServiceContext
	genGID             func(string) string
	submitSaga         func(*dtmgrpc.SagaGrpc) error
	loadSnapshotResult func(string) (*types.CreateOrderResp, error)
}

func NewCreateOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateOrderLogic {
	logic := &CreateOrderLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
	logic.genGID = dtmgrpc.MustGenGid
	logic.submitSaga = func(saga *dtmgrpc.SagaGrpc) error {
		return saga.Submit()
	}
	logic.loadSnapshotResult = func(orderID string) (*types.CreateOrderResp, error) {
		return logic.queryCreateOrderResp(orderID)
	}
	return logic
}

func (l *CreateOrderLogic) CreateOrder(req *types.CreateOrderReq) (resp *types.CreateOrderResp, err error) {
	requestId := strings.TrimSpace(req.RequestId)
	fields := commonobs.OrderFields(l.ctx, "", requestId)
	fields = append(fields,
		logx.Field("user_id", req.UserId),
		logx.Field("product_id", req.ProductId),
		logx.Field("amount", req.Amount),
		logx.Field("expected_price_fen", req.ExpectedPriceFen),
	)
	l.Infow("create order request", fields...)

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
			if err := l.svcCtx.Redis.SetexCtx(l.ctx, requestResultKey, existOrder.Id, int(requestTTLSeconds)); err != nil {
				l.Errorf("set request result cache failed: request_id=%s order_id=%s err=%v", requestId, existOrder.Id, err)
			}
			l.releaseRequestLock(requestLockKey, requestId)

			l.Infow("request_id hit db cache", commonobs.OrderFields(l.ctx, existOrder.Id, requestId)...)
			result = "hit"
			return l.loadSnapshotResult(existOrder.Id)
		} else if lookupErr != model.ErrNotFound {
			return nil, lookupErr
		}

		if cachedOrderId, _ := l.svcCtx.Redis.GetCtx(l.ctx, requestResultKey); cachedOrderId != "" {
			l.Infow("request_id hit redis cache", commonobs.OrderFields(l.ctx, cachedOrderId, requestId)...)
			result = "hit"
			return l.loadSnapshotResult(cachedOrderId)
		}
	}

	if l.svcCtx.Config.DtmServer == "" {
		return nil, status.Error(codes.Internal, "DTM server not configured")
	}
	if l.svcCtx.Config.OrderRpcTarget == "" {
		return nil, status.Error(codes.Internal, "Order RPC target not configured")
	}
	if !l.svcCtx.Config.InventoryOwnsFinalDeduct && l.svcCtx.Config.ProductRpcTarget == "" {
		return nil, status.Error(codes.Internal, "Product RPC target not configured")
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
			l.Infow("request_id locked, return existing", commonobs.OrderFields(l.ctx, cachedOrderId, requestId)...)
			result = "hit"
			return l.loadSnapshotResult(cachedOrderId)
		}
		return nil, status.Error(codes.Aborted, "duplicate request in progress")
	}

	if orderID == "" {
		orderID = l.svcCtx.OrderIdGen.NextID()
	}
	gid := l.genGID(l.svcCtx.Config.DtmServer)

	saga := dtmgrpc.NewSagaGrpc(l.svcCtx.Config.DtmServer, gid)
	saga.WaitResult = true
	if l.svcCtx.Config.DtmTimeoutToFailSeconds > 0 {
		saga.TimeoutToFail = l.svcCtx.Config.DtmTimeoutToFailSeconds
	}
	if l.svcCtx.Config.DtmRequestTimeoutSeconds > 0 {
		saga.WithGlobalTransRequestTimeout(l.svcCtx.Config.DtmRequestTimeoutSeconds)
	}
	if l.svcCtx.Config.DtmWaitResult {
		saga.WaitResult = true
	}

	orderRoute := l.svcCtx.Config.OrderRpcTarget + "/order.Order"

	preDeductReq := &order.PreDeductReq{
		ProductId: req.ProductId,
		Amount:    req.Amount,
		OrderId:   orderID,
	}
	createOrderReq := &order.CreateOrderReq{
		OrderId:          orderID,
		RequestId:        requestId,
		UserId:           req.UserId,
		ProductId:        req.ProductId,
		Amount:           req.Amount,
		ExpectedPriceFen: req.ExpectedPriceFen,
	}

	saga.Add(orderRoute+"/PreDeduct", orderRoute+"/PreDeductRollback", preDeductReq)
	saga.Add(orderRoute+"/CreateOrder", orderRoute+"/CreateOrderRollback", createOrderReq)
	if !l.svcCtx.Config.InventoryOwnsFinalDeduct {
		productRoute := l.svcCtx.Config.ProductRpcTarget + "/product.Product"
		deductReq := &product.DeductReq{
			Id:      req.ProductId,
			Num:     req.Amount,
			OrderId: orderID,
		}
		saga.Add(productRoute+"/Deduct", productRoute+"/DeductRollback", deductReq)
	}

	if err = l.submitSaga(saga); err != nil {
		l.Errorf("submit SAGA failed: %v", err)
		metrics.OrderSagaSubmitTotal.WithLabelValues("fail").Inc()
		l.releaseRequestLock(requestLockKey, requestId)
		return nil, status.Error(codes.Unavailable, "order system busy")
	}
	metrics.OrderSagaSubmitTotal.WithLabelValues("success").Inc()

	if setResultErr := l.svcCtx.Redis.SetexCtx(l.ctx, requestResultKey, orderID, int(requestTTLSeconds)); setResultErr != nil {
		l.Errorf("set request result key failed: %v, request_id=%s, order_id=%s", setResultErr, requestId, orderID)
	} else {
		l.releaseRequestLock(requestLockKey, requestId)
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
	return l.loadSnapshotResult(orderID)
}

type createOrderSnapshotRow struct {
	OrderID          string `db:"order_id"`
	Status           int64  `db:"status"`
	PayableAmountFen int64  `db:"payable_amount_fen"`
	PaymentOrderID   string `db:"payment_order_id"`
}

func (l *CreateOrderLogic) queryCreateOrderResp(orderID string) (*types.CreateOrderResp, error) {
	if strings.TrimSpace(orderID) == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}
	if l.svcCtx.SqlConn == nil {
		return &types.CreateOrderResp{
			OrderId:        orderID,
			Status:         orderStatusText(0),
			PaymentOrderId: paymentOrderIDFor(orderID),
		}, nil
	}

	var row createOrderSnapshotRow
	err := l.svcCtx.SqlConn.QueryRowCtx(l.ctx, &row, `
SELECT o.id AS order_id,
       o.status AS status,
       s.payable_amount_fen AS payable_amount_fen,
       p.id AS payment_order_id
FROM orders o
JOIN order_price_snapshot s ON s.order_id = o.id
JOIN payment_order p ON p.order_id = o.id
WHERE o.id = ?
LIMIT 1`, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "order snapshot not found")
		}
		return nil, err
	}

	return &types.CreateOrderResp{
		OrderId:          row.OrderID,
		Status:           orderStatusText(row.Status),
		PayableAmountFen: row.PayableAmountFen,
		PaymentOrderId:   row.PaymentOrderID,
	}, nil
}

func (l *CreateOrderLogic) releaseRequestLock(lockKey, requestID string) {
	if _, err := l.svcCtx.Redis.DelCtx(l.ctx, lockKey); err != nil {
		l.Errorf("release request lock failed: request_id=%s err=%v", requestID, err)
	}
}

func orderStatusText(statusCode int64) string {
	return orderstatus.Text(statusCode)
}

func paymentOrderIDFor(orderID string) string {
	return "pay:" + orderID
}
