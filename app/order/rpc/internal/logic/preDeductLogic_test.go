package logic

import (
	"context"
	"fmt"
	"testing"

	"flash-mall/app/order/rpc/internal/config"
	"flash-mall/app/order/rpc/internal/svc"
	order "flash-mall/app/order/rpc/order"
	productclient "flash-mall/app/product/rpc/productclient"

	"github.com/alicebob/miniredis/v2"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"google.golang.org/grpc"
)

func TestPreDeductLogic_PreDeduct(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rds := redis.MustNewRedis(redis.RedisConf{
		Host: mr.Addr(),
		Type: redis.NodeType,
	})

	svcCtx := &svc.ServiceContext{
		Redis:  rds,
		Config: config.Config{StockShardCount: 2},
	}
	l := NewPreDeductLogic(context.Background(), svcCtx)

	productId := int64(1001)
	shardCount := stockShardCount(svcCtx.Config.StockShardCount)
	stockKeys := stockShardKeys(productId, shardCount)

	// case 1: stock not initialized
	_, err = l.PreDeduct(&order.PreDeductReq{ProductId: productId, Amount: 1, OrderId: "o1"})
	if err == nil {
		t.Fatal("expected error for uninitialized stock")
	}

	// case 2: insufficient stock
	for _, key := range stockKeys {
		_ = mr.Set(key, "1")
	}
	_, err = l.PreDeduct(&order.PreDeductReq{ProductId: productId, Amount: 2, OrderId: "o2"})
	if err == nil {
		t.Fatal("expected error for insufficient stock")
	}

	// case 3: success
	for _, key := range stockKeys {
		_ = mr.Set(key, "10")
	}
	_, err = l.PreDeduct(&order.PreDeductReq{ProductId: productId, Amount: 5, OrderId: "o3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var total int64
	for _, key := range stockKeys {
		val, _ := mr.Get(key)
		if val == "" {
			continue
		}
		var v int64
		_, _ = fmt.Sscan(val, &v)
		total += v
	}
	if total != int64(len(stockKeys))*10-5 {
		t.Fatalf("expected total stock=%d, got %d", int64(len(stockKeys))*10-5, total)
	}
}

func TestPreDeductLogic_SelfHealsMissingRedisStock(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rds := redis.MustNewRedis(redis.RedisConf{
		Host: mr.Addr(),
		Type: redis.NodeType,
	})

	svcCtx := &svc.ServiceContext{
		Redis:      rds,
		ProductRpc: &preDeductProductStub{stock: 7},
		Config:     config.Config{StockShardCount: 2},
	}
	l := NewPreDeductLogic(context.Background(), svcCtx)

	productId := int64(1002)
	_, err = l.PreDeduct(&order.PreDeductReq{ProductId: productId, Amount: 3, OrderId: "self-heal-order"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var total int64
	for _, key := range stockShardKeys(productId, 2) {
		val, _ := mr.Get(key)
		if val == "" {
			t.Fatalf("expected redis stock key %s to be initialized", key)
		}
		var v int64
		_, _ = fmt.Sscan(val, &v)
		total += v
	}
	if total != 4 {
		t.Fatalf("expected total stock after self-heal pre-deduct = 4, got %d", total)
	}
}

type preDeductProductStub struct {
	stock int64
}

func (s *preDeductProductStub) GetProductCard(context.Context, *productclient.GetProductCardReq, ...grpc.CallOption) (*productclient.GetProductCardResp, error) {
	return &productclient.GetProductCardResp{StockAvailable: s.stock}, nil
}

func (s *preDeductProductStub) ListProducts(context.Context, *productclient.ListProductsReq, ...grpc.CallOption) (*productclient.ListProductsResp, error) {
	panic("unexpected ListProducts call")
}

func (s *preDeductProductStub) Deduct(context.Context, *productclient.DeductReq, ...grpc.CallOption) (*productclient.Empty, error) {
	panic("unexpected Deduct call")
}

func (s *preDeductProductStub) DeductRollback(context.Context, *productclient.DeductReq, ...grpc.CallOption) (*productclient.Empty, error) {
	panic("unexpected DeductRollback call")
}

func (s *preDeductProductStub) RevertStock(context.Context, *productclient.RevertStockReq, ...grpc.CallOption) (*productclient.RevertStockResp, error) {
	panic("unexpected RevertStock call")
}

func TestPreDeductRollbackLogic_PreDeductRollback(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rds := redis.MustNewRedis(redis.RedisConf{
		Host: mr.Addr(),
		Type: redis.NodeType,
	})

	svcCtx := &svc.ServiceContext{
		Redis:  rds,
		Config: config.Config{StockShardCount: 2},
	}
	l := NewPreDeductRollbackLogic(context.Background(), svcCtx)

	productId := int64(1001)
	shardCount := stockShardCount(svcCtx.Config.StockShardCount)
	stockKeys := stockShardKeys(productId, shardCount)
	for _, key := range stockKeys {
		_ = mr.Set(key, "5")
	}

	// rollback without try (empty compensation)
	_, err = l.PreDeductRollback(&order.PreDeductReq{ProductId: productId, Amount: 3, OrderId: "o1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, _ := mr.Get(stockKeys[0])
	if val != "5" {
		t.Fatalf("expected stock=5 after empty compensation, got %s", val)
	}

	// simulate try by setting try key
	tryKey := "order:pre_deduct:try:o2"
	_ = mr.Set(tryKey, "1")
	_, err = l.PreDeductRollback(&order.PreDeductReq{ProductId: productId, Amount: 2, OrderId: "o2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, _ = mr.Get(stockKeys[0])
	if val != "7" {
		t.Fatalf("expected stock=7 after rollback, got %s", val)
	}

	// idempotent rollback (second time should not change)
	_, err = l.PreDeductRollback(&order.PreDeductReq{ProductId: productId, Amount: 2, OrderId: "o2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, _ = mr.Get(stockKeys[0])
	if val != "7" {
		t.Fatalf("expected stock=7 after idempotent rollback, got %s", val)
	}
}
