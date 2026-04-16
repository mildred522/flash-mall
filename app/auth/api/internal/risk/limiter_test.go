package risk

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func TestMemoryLimiter_BlocksAtThreshold(t *testing.T) {
	limiter := NewMemoryLimiter()
	ctx := context.Background()
	key := "auth:risk:login:phone:13800000001"
	for i := 0; i < 5; i++ {
		if err := limiter.Incr(ctx, key, time.Minute); err != nil {
			t.Fatalf("incr failed: %v", err)
		}
	}
	blocked, count, err := limiter.Blocked(ctx, key, 5)
	if err != nil || !blocked || count != 5 {
		t.Fatalf("expected blocked at count 5, got blocked=%v count=%d err=%v", blocked, count, err)
	}
}

func TestRedisLimiter_BlocksAtThreshold(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	limiter := NewRedisLimiter(redis.MustNewRedis(redis.RedisConf{
		Host: mr.Addr(),
		Type: redis.NodeType,
	}))

	ctx := context.Background()
	key := "auth:risk:login:phone:13800000001"
	for i := 0; i < 5; i++ {
		if err := limiter.Incr(ctx, key, time.Minute); err != nil {
			t.Fatalf("incr failed: %v", err)
		}
	}
	blocked, count, err := limiter.Blocked(ctx, key, 5)
	if err != nil || !blocked || count != 5 {
		t.Fatalf("expected blocked at count 5, got blocked=%v count=%d err=%v", blocked, count, err)
	}
	if got, _ := mr.Get(key); got != "5" {
		t.Fatalf("expected redis count 5, got %q", got)
	}
}

func TestRedisLimiter_Reset(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	limiter := NewRedisLimiter(redis.MustNewRedis(redis.RedisConf{
		Host: mr.Addr(),
		Type: redis.NodeType,
	}))

	ctx := context.Background()
	key := "auth:risk:login:ip:203.0.113.9"
	if err := limiter.Incr(ctx, key, time.Minute); err != nil {
		t.Fatalf("incr failed: %v", err)
	}
	if err := limiter.Reset(ctx, key); err != nil {
		t.Fatalf("reset failed: %v", err)
	}
	blocked, count, err := limiter.Blocked(ctx, key, 1)
	if err != nil || blocked || count != 0 {
		t.Fatalf("expected reset key to disappear, got blocked=%v count=%d err=%v", blocked, count, err)
	}
}
