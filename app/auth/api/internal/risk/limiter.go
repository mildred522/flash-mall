package risk

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

type Limiter interface {
	Incr(ctx context.Context, key string, ttl time.Duration) error
	Blocked(ctx context.Context, key string, max int64) (bool, int64, error)
	Reset(ctx context.Context, key string) error
}

type MemoryLimiter struct {
	mu     sync.Mutex
	counts map[string]limiterEntry
}

type RedisLimiter struct {
	rds *redis.Redis
}

type limiterEntry struct {
	count     int64
	expiresAt time.Time
}

func NewMemoryLimiter() *MemoryLimiter {
	return &MemoryLimiter{
		counts: make(map[string]limiterEntry),
	}
}

func (l *MemoryLimiter) Incr(_ context.Context, key string, ttl time.Duration) error {
	if key == "" {
		return nil
	}
	if ttl <= 0 {
		ttl = time.Minute
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry := l.counts[key]
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		entry.count = 0
	}
	entry.count++
	entry.expiresAt = time.Now().Add(ttl)
	l.counts[key] = entry
	return nil
}

func (l *MemoryLimiter) Blocked(_ context.Context, key string, max int64) (bool, int64, error) {
	if key == "" {
		return false, 0, nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry := l.counts[key]
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		delete(l.counts, key)
		return false, 0, nil
	}
	return entry.count >= max && max > 0, entry.count, nil
}

func (l *MemoryLimiter) Reset(_ context.Context, key string) error {
	if key == "" {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.counts, key)
	return nil
}

func NewRedisLimiter(rds *redis.Redis) *RedisLimiter {
	return &RedisLimiter{rds: rds}
}

func (l *RedisLimiter) Incr(ctx context.Context, key string, ttl time.Duration) error {
	if l == nil || l.rds == nil || key == "" {
		return nil
	}
	if ttl <= 0 {
		ttl = time.Minute
	}
	ctx, cancel := withRedisTimeout(ctx)
	defer cancel()

	if _, err := l.rds.IncrbyCtx(ctx, key, 1); err != nil {
		return err
	}
	return l.rds.ExpireCtx(ctx, key, int(ttl.Seconds()))
}

func (l *RedisLimiter) Blocked(ctx context.Context, key string, max int64) (bool, int64, error) {
	if l == nil || l.rds == nil || key == "" || max <= 0 {
		return false, 0, nil
	}
	ctx, cancel := withRedisTimeout(ctx)
	defer cancel()

	raw, err := l.rds.GetCtx(ctx, key)
	if err != nil {
		return false, 0, err
	}
	if raw == "" {
		return false, 0, nil
	}
	count, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return false, 0, err
	}
	return count >= max, count, nil
}

func (l *RedisLimiter) Reset(ctx context.Context, key string) error {
	if l == nil || l.rds == nil || key == "" {
		return nil
	}
	ctx, cancel := withRedisTimeout(ctx)
	defer cancel()
	_, err := l.rds.DelCtx(ctx, key)
	return err
}

func withRedisTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.WithTimeout(context.Background(), 150*time.Millisecond)
	}
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= 150*time.Millisecond {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, 150*time.Millisecond)
}
