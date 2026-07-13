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

type PreDeductRollbackLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPreDeductRollbackLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PreDeductRollbackLogic {
	return &PreDeductRollbackLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PreDeductRollback 是 Redis 预扣的 DTM 补偿分支。
func (l *PreDeductRollbackLogic) PreDeductRollback(in *order.PreDeductReq) (*order.Empty, error) {
	if in.OrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}
	shardCount := stockShardCount(l.svcCtx.Config.StockShardCount)
	stockKeys := stockShardKeys(in.ProductId, shardCount)
	l.Infow("pre-deduct rollback start",
		logx.Field("order_id", in.OrderId),
		logx.Field("product_id", in.ProductId),
		logx.Field("amount", in.Amount),
		logx.Field("shards", shardCount),
	)
	tryKey := fmt.Sprintf("order:pre_deduct:try:%s", in.OrderId)
	rollbackKey := fmt.Sprintf("order:pre_deduct:rollback:%s", in.OrderId)

	// CHG 2026-01-31: 变更=回滚幂等且支持空补偿; 之前=无幂等/空补偿处理; 原因=避免重复回滚与悬挂。
	// CHG 2026-02-07: 变更=按 tryKey 记录的分片回滚; 之前=单 key 回滚; 原因=分片后保证归还到正确分片。
	const luaScript = `
        local shardCount = tonumber(ARGV[3])
        local tryKey = KEYS[shardCount + 1]
        local rollbackKey = KEYS[shardCount + 2]
        if redis.call("exists", rollbackKey) == 1 then
            return 1 -- already rolled back
        end
        local shardIndex = redis.call("get", tryKey)
        if not shardIndex then
            redis.call("set", rollbackKey, 1)
            local ttl = tonumber(ARGV[2])
            if ttl and ttl > 0 then
                redis.call("expire", rollbackKey, ttl)
            end
            return 1 -- empty compensation
        end
        local amount = tonumber(ARGV[1])
        local idx = tonumber(shardIndex)
        local stockKey = KEYS[idx]
        if stockKey then
            redis.call("incrby", stockKey, amount)
        end
        redis.call("set", rollbackKey, 1)
        local ttl = tonumber(ARGV[2])
        if ttl and ttl > 0 then
            redis.call("expire", rollbackKey, ttl)
        end
        return 1
    `

	keys := append(stockKeys, tryKey, rollbackKey)
	_, err := l.svcCtx.Redis.EvalCtx(l.ctx, luaScript, keys, in.Amount, preDeductKeyTTLSeconds, shardCount)
	if err != nil {
		l.Errorf("redis pre-deduct rollback failed: %v", err)
		return nil, status.Error(codes.Internal, "redis pre-deduct rollback failed")
	}

	return &order.Empty{}, nil
}
