package logic

import (
	"context"
	"fmt"
	"testing"

	"flash-mall/app/order/api/internal/svc"
	"flash-mall/app/order/api/internal/types"

	"github.com/alicebob/miniredis/v2"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func TestCreateOrderLogic_CreateOrder_RedisLimit(t *testing.T) {
	// 1. Setup mock Redis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	// 2. Setup ServiceContext with mock Redis
	// We construct RedisConf manually to point to miniredis
	rds := redis.MustNewRedis(redis.RedisConf{
		Host: mr.Addr(),
		Type: redis.NodeType,
	})

	svcCtx := &svc.ServiceContext{
		Redis: rds,
		// ProductRpc is nil, which might cause panic if code reaches DTM part,
		// but we are testing Redis part first.
	}

	// 3. Setup Logic
	ctx := context.Background()
	l := NewCreateOrderLogic(ctx, svcCtx)

	productId := int64(1001)
	stockKey := fmt.Sprintf("stock:%d", productId)

	// CHG 2026-01-31: 变更=Redis 预扣迁移到 order-rpc 的 SAGA 分支;
	// 之前=API 里直接预扣; 原因=统一补偿与幂等处理。
	// 这里仅验证：API 在未配置 DTM 时不会改动 Redis。

	mr.Set(stockKey, "10")
	req := &types.CreateOrderReq{
		UserId:    1,
		ProductId: productId,
		Amount:    5,
	}

	_, err = l.CreateOrder(req)
	if err == nil {
		t.Error("expected error when DTM server is not configured, got nil")
	}

	val, _ := mr.Get(stockKey)
	if val != "10" {
		t.Errorf("expected stock to remain 10, got %s", val)
	}
}
