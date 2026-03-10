package job

import (
	"context"
	"errors"
	"time"

	"flash-mall/app/order/api/internal/metrics"
	"flash-mall/app/order/api/internal/svc"
	orderrpc "flash-mall/app/order/rpc/order"
	"flash-mall/app/product/rpc/product"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	OrderDelayQueueKey       = "order:delay:queue"
	OrderProcessingQueueKey  = "order:delay:processing"
	OrderRetryKeyPrefix      = "order:delay:retry:"
	OrderDLQKey              = "order:delay:dlq"
	visibilityTimeoutSeconds = 30
	maxRetries               = 5
	retryBackoffSeconds      = 5
)

type CloseOrderJob struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCloseOrderJob(svcCtx *svc.ServiceContext) *CloseOrderJob {
	return &CloseOrderJob{
		ctx:    context.Background(),
		svcCtx: svcCtx,
		Logger: logx.WithContext(context.Background()),
	}
}

// Start 启动延迟队列消费循环。
func (j *CloseOrderJob) Start() {
	j.Info("delay-queue consumer started")

	go func() {
		for {
			j.consume()
			time.Sleep(1 * time.Second)
		}
	}()
}

func (j *CloseOrderJob) consume() {
	now := time.Now().Unix()

	// CHG 2026-01-31: 变更=回收处理队列超时任务; 之前=无可见性超时回收; 原因=防止消费者崩溃丢单。
	_ = j.reclaimExpired(now)
	j.updateBacklogMetrics()

	orderId, err := j.claimDueOrder(now)
	if err != nil {
		j.Errorf("claim due order failed: %v", err)
		return
	}
	if orderId == "" {
		return
	}

	if err := j.handleCloseOrder(orderId); err != nil {
		j.onFailure(orderId, err)
		return
	}

	j.ack(orderId)
}

// claimDueOrder 原子领取到期任务（delay -> processing）。
func (j *CloseOrderJob) claimDueOrder(now int64) (string, error) {
	const claimLua = `
        local items = redis.call("ZRANGEBYSCORE", KEYS[1], 0, ARGV[1], "LIMIT", 0, 1)
        if #items == 0 then
            return ""
        end
        local orderId = items[1]
        redis.call("ZREM", KEYS[1], orderId)
        redis.call("ZADD", KEYS[2], ARGV[2], orderId)
        return orderId
    `

	visibilityAt := now + visibilityTimeoutSeconds
	val, err := j.svcCtx.Redis.EvalCtx(j.ctx, claimLua, []string{OrderDelayQueueKey, OrderProcessingQueueKey}, now, visibilityAt)
	if err != nil {
		return "", err
	}

	orderId, _ := val.(string)
	return orderId, nil
}

// reclaimExpired 回收超时任务回到延迟队列重试。
func (j *CloseOrderJob) reclaimExpired(now int64) error {
	const reclaimLua = `
        local items = redis.call("ZRANGEBYSCORE", KEYS[1], 0, ARGV[1], "LIMIT", 0, ARGV[2])
        for i, v in ipairs(items) do
            redis.call("ZREM", KEYS[1], v)
            redis.call("ZADD", KEYS[2], ARGV[1], v)
        end
        return #items
    `

	_, err := j.svcCtx.Redis.EvalCtx(j.ctx, reclaimLua, []string{OrderProcessingQueueKey, OrderDelayQueueKey}, now, 10)
	return err
}

func (j *CloseOrderJob) ack(orderId string) {
	_, _ = j.svcCtx.Redis.ZremCtx(j.ctx, OrderProcessingQueueKey, orderId)
	_, _ = j.svcCtx.Redis.DelCtx(j.ctx, OrderRetryKeyPrefix+orderId)
}

func (j *CloseOrderJob) onFailure(orderId string, err error) {
	retryKey := OrderRetryKeyPrefix + orderId
	retryCount, _ := j.svcCtx.Redis.IncrbyCtx(j.ctx, retryKey, 1)
	_ = j.svcCtx.Redis.ExpireCtx(j.ctx, retryKey, 24*60*60)

	if retryCount > maxRetries {
		// CHG 2026-01-31: 变更=超过重试上限进入死信队列; 之前=无死信隔离; 原因=避免无限重试。
		_, _ = j.svcCtx.Redis.ZaddCtx(j.ctx, OrderDLQKey, time.Now().Unix(), orderId)
		metrics.OrderCompensationTotal.WithLabelValues("dlq").Inc()
		j.Errorf("order moved to DLQ: %s, err=%v", orderId, err)
		j.ack(orderId)
		return
	}

	// CHG 2026-01-31: 变更=失败后退避重试; 之前=无重试策略; 原因=提升成功率并避免击穿。
	nextAt := time.Now().Unix() + retryBackoffSeconds
	_, _ = j.svcCtx.Redis.ZaddCtx(j.ctx, OrderDelayQueueKey, nextAt, orderId)
	j.ack(orderId)
}

// handleCloseOrder 处理关单逻辑，返回错误触发重试。
func (j *CloseOrderJob) handleCloseOrder(orderId string) error {
	// 1) 从 DB 加载订单。
	order, err := j.svcCtx.OrderModel.FindOne(j.ctx, orderId)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			j.Infof("order not found, skip: %s", orderId)
			return nil
		}
		return err
	}

	// 2) 仅处理未支付订单。
	if order.Status != 0 {
		j.Infof("order status=%d, skip: %s", order.Status, orderId)
		return nil
	}

	// 3) 通过 RPC 归还库存。
	_, err = j.svcCtx.ProductRpc.RevertStock(j.ctx, &product.RevertStockReq{
		Id:      order.ProductId,
		Num:     order.Amount,
		OrderId: orderId,
	})
	if err != nil {
		return err
	}

	// CHG 2026-02-24: 变更=关单时回滚 Redis 预扣库存; 之前=仅回补 DB; 原因=避免 Redis/DB 库存漂移。
	_, err = j.svcCtx.OrderRpc.PreDeductRollback(j.ctx, &orderrpc.PreDeductReq{
		ProductId: order.ProductId,
		Amount:    order.Amount,
		OrderId:   orderId,
	})
	if err != nil {
		return err
	}

	// 4) 更新订单状态为已关闭。
	order.Status = 2
	if err := j.svcCtx.OrderModel.Update(j.ctx, order); err != nil {
		return err
	}

	metrics.OrderCompensationTotal.WithLabelValues("close").Inc()
	j.Infof("order closed: %s", orderId)
	return nil
}

func (j *CloseOrderJob) updateBacklogMetrics() {
	delayCount, _ := j.svcCtx.Redis.ZcardCtx(j.ctx, OrderDelayQueueKey)
	processingCount, _ := j.svcCtx.Redis.ZcardCtx(j.ctx, OrderProcessingQueueKey)
	metrics.DelayQueueBacklog.Set(float64(delayCount + processingCount))
}
