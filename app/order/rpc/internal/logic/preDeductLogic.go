package logic

import (
	"context"
	"fmt"

	"flash-mall/app/order/rpc/internal/svc"
	order "flash-mall/app/order/rpc/order"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	preDeductKeyTTLSeconds = 24 * 60 * 60
)

type PreDeductLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPreDeductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PreDeductLogic {
	return &PreDeductLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PreDeduct 是 Redis 预扣库存的 DTM SAGA 分支。
func (l *PreDeductLogic) PreDeduct(in *order.PreDeductReq) (*order.Empty, error) {
	if in.OrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}
	shardCount := stockShardCount(l.svcCtx.Config.StockShardCount)
	stockKeys := stockShardKeys(in.ProductId, shardCount)
	l.Infow("pre-deduct start",
		logx.Field("order_id", in.OrderId),
		logx.Field("product_id", in.ProductId),
		logx.Field("amount", in.Amount),
		logx.Field("shards", shardCount),
	)
	tryKey := fmt.Sprintf("order:pre_deduct:try:%s", in.OrderId)
	rollbackKey := fmt.Sprintf("order:pre_deduct:rollback:%s", in.OrderId)

	// CHG 2026-01-31: 变更=Redis 预扣下沉为 DTM 分支并加幂等键; 之前=API 内直接预扣; 原因=保证补偿与幂等。
	// CHG 2026-02-07: 变更=库存 key 分片 + Lua 内部挑选可用分片; 之前=单 key 热点; 原因=分散高并发热点。
	const luaScript = `
        local shardCount = tonumber(ARGV[3])
        local tryKey = KEYS[shardCount + 1]
        local rollbackKey = KEYS[shardCount + 2]
        if redis.call("exists", rollbackKey) == 1 then
            return 1 -- rollback already happened, no-op
        end
        if redis.call("exists", tryKey) == 1 then
            return 1 -- idempotent retry
        end
        local amount = tonumber(ARGV[1])
        local ttl = tonumber(ARGV[2])
        local start = tonumber(ARGV[4])
        local hasStockKey = false
        for i = 0, shardCount - 1 do
            local idx = ((start + i) % shardCount) + 1
            local stockKey = KEYS[idx]
            local stock = redis.call("get", stockKey)
            if stock then
                hasStockKey = true
                stock = tonumber(stock)
                if stock >= amount then
                    redis.call("decrby", stockKey, amount)
                    redis.call("set", tryKey, idx)
                    if ttl and ttl > 0 then
                        redis.call("expire", tryKey, ttl)
                    end
                    return stock - amount
                end
            end
        end
        if hasStockKey == false then
            return -1 -- not initialized
        end
        return -2 -- insufficient
    `

	startIndex := stockShardStartIndex(in.OrderId, shardCount)
	keys := append(stockKeys, tryKey, rollbackKey)
	val, err := l.svcCtx.Redis.EvalCtx(l.ctx, luaScript, keys, in.Amount, preDeductKeyTTLSeconds, shardCount, startIndex)
	if err != nil {
		l.Errorf("redis pre-deduct failed: %v", err)
		return nil, status.Error(codes.Internal, "redis pre-deduct failed")
	}

	ret, _ := val.(int64)
	switch ret {
	case -1:
		return nil, status.Error(codes.FailedPrecondition, "stock not initialized")
	case -2:
		return nil, status.Error(codes.Aborted, "insufficient stock")
	}

	return &order.Empty{}, nil
}
