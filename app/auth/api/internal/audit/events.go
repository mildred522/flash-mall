package audit

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

type Event struct {
	EventType     string
	Result        string
	UserID        int64
	IdentityValue string
	IP            string
	UserAgent     string
	CreatedAt     time.Time
}

type Recorder interface {
	Record(ctx context.Context, event Event) error
	ListRecent(ctx context.Context, limit int) ([]Event, error)
}

type MemoryRecorder struct {
	mu     sync.Mutex
	limit  int
	events []Event
}

type RedisRecorder struct {
	rds   *redis.Redis
	limit int
}

const (
	redisEventsKey        = "auth:audit:events"
	redisOperationTimeout = time.Second
)

func NewMemoryRecorder(limit int) *MemoryRecorder {
	if limit <= 0 {
		limit = 10
	}
	return &MemoryRecorder{limit: limit}
}

func (r *MemoryRecorder) Record(_ context.Context, event Event) error {
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.events = append(r.events, event)
	if len(r.events) > r.limit {
		r.events = append([]Event(nil), r.events[len(r.events)-r.limit:]...)
	}
	return nil
}

func (r *MemoryRecorder) ListRecent(_ context.Context, limit int) ([]Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if limit <= 0 || limit > len(r.events) {
		limit = len(r.events)
	}
	out := make([]Event, 0, limit)
	for i := len(r.events) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, r.events[i])
	}
	return out, nil
}

func NewRedisRecorder(rds *redis.Redis, limit int) *RedisRecorder {
	if limit <= 0 {
		limit = 10
	}
	return &RedisRecorder{rds: rds, limit: limit}
}

func (r *RedisRecorder) Record(ctx context.Context, event Event) error {
	if r == nil || r.rds == nil {
		return nil
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	ctx, cancel := withRedisTimeout(ctx)
	defer cancel()
	return r.rds.PipelinedCtx(ctx, func(pipe redis.Pipeliner) error {
		pipe.LPush(ctx, redisEventsKey, string(payload))
		pipe.LTrim(ctx, redisEventsKey, 0, int64(r.limit-1))
		return nil
	})
}

func (r *RedisRecorder) ListRecent(ctx context.Context, limit int) ([]Event, error) {
	if r == nil || r.rds == nil {
		return nil, nil
	}
	if limit <= 0 || limit > r.limit {
		limit = r.limit
	}
	ctx, cancel := withRedisTimeout(ctx)
	defer cancel()
	items, err := r.rds.LrangeCtx(ctx, redisEventsKey, 0, limit-1)
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0, len(items))
	for _, raw := range items {
		var event Event
		if err := json.Unmarshal([]byte(raw), &event); err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	return out, nil
}

func withRedisTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.WithTimeout(context.Background(), redisOperationTimeout)
	}
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= redisOperationTimeout {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, redisOperationTimeout)
}
