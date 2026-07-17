package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"time"

	redis "github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

type Source string

const (
	SourceL1     Source = "l1"
	SourceL2     Source = "l2"
	SourceStale  Source = "stale"
	SourceOrigin Source = "origin"
)

type Config struct {
	Prefix               string
	EnableL1             bool
	EnableL2             bool
	StaleWhileRevalidate bool
	AllowStaleOnError    bool
	L1TTL                time.Duration
	SoftTTL              time.Duration
	HardTTL              time.Duration
	OperationTimeout     time.Duration
	MaxEntries           int
}

type Coordinator struct {
	config Config
	redis  *redis.Client
	local  *localCache
	group  singleflight.Group
	mu     sync.Mutex
	pubsub *redis.PubSub
	cancel context.CancelFunc
}

type envelope struct {
	Value         json.RawMessage `json:"value"`
	SoftExpiresAt int64           `json:"soft_expires_at"`
	HardExpiresAt int64           `json:"hard_expires_at"`
}

type loadResult struct {
	value  []byte
	source Source
}

func New(config Config, client *redis.Client) *Coordinator {
	config.Prefix = strings.Trim(strings.TrimSpace(config.Prefix), ":")
	if config.Prefix == "" {
		config.Prefix = "flashmall:cache"
	}
	if config.L1TTL <= 0 {
		config.L1TTL = 2 * time.Second
	}
	if config.SoftTTL <= 0 {
		config.SoftTTL = 30 * time.Second
	}
	if config.HardTTL <= config.SoftTTL {
		config.HardTTL = 2 * config.SoftTTL
	}
	if config.OperationTimeout <= 0 {
		config.OperationTimeout = 200 * time.Millisecond
	}
	if client == nil {
		config.EnableL2 = false
	}
	return &Coordinator{config: config, redis: client, local: newLocalCache(config.MaxEntries)}
}

func (c *Coordinator) GetOrLoad(ctx context.Context, key string, loader func(context.Context) ([]byte, error)) ([]byte, Source, error) {
	now := time.Now()
	if c.config.EnableL1 {
		if value, ok := c.local.get(key, now); ok {
			recordLookup("l1", "hit")
			return value, SourceL1, nil
		}
		recordLookup("l1", "miss")
	}
	var stale []byte
	if c.config.EnableL2 {
		cached, err := c.readL2(ctx, key, now)
		if err == nil && cached != nil {
			if now.UnixMilli() < cached.SoftExpiresAt {
				recordLookup("l2", "hit")
				c.setL1(key, cached.Value, now)
				return cloneBytes(cached.Value), SourceL2, nil
			}
			stale = cloneBytes(cached.Value)
			if c.config.StaleWhileRevalidate {
				recordLookup("l2", "stale")
				c.refreshAsync(ctx, key, loader)
				return stale, SourceStale, nil
			}
		}
		if err != nil && err != redis.Nil {
			recordLookup("l2", "error")
		} else {
			recordLookup("l2", "miss")
		}
	}
	value, err, _ := c.group.Do(key, func() (any, error) {
		loaded, loadErr := loader(ctx)
		if loadErr != nil {
			return nil, loadErr
		}
		if err := c.store(ctx, key, loaded); err != nil && c.config.EnableL2 {
			// Cache availability must never become origin availability. The
			// successful value remains usable and L2 can recover independently.
			recordLookup("l2", "write_error")
		}
		return loadResult{value: cloneBytes(loaded), source: SourceOrigin}, nil
	})
	if err != nil {
		recordLookup("origin", "error")
		if len(stale) > 0 && c.config.AllowStaleOnError {
			return stale, SourceStale, nil
		}
		return nil, "", err
	}
	recordLookup("origin", "success")
	result := value.(loadResult)
	return result.value, result.source, nil
}

func (c *Coordinator) readL2(ctx context.Context, key string, now time.Time) (*envelope, error) {
	opCtx, cancel := c.operationContext(ctx)
	defer cancel()
	raw, err := c.redis.Get(opCtx, c.redisKey(key)).Bytes()
	if err != nil {
		return nil, err
	}
	var cached envelope
	if err := json.Unmarshal(raw, &cached); err != nil {
		return nil, err
	}
	if cached.HardExpiresAt <= now.UnixMilli() {
		_ = c.redis.Del(opCtx, c.redisKey(key)).Err()
		return nil, redis.Nil
	}
	return &cached, nil
}

func (c *Coordinator) store(ctx context.Context, key string, value []byte) error {
	now := time.Now()
	c.setL1(key, value, now)
	if !c.config.EnableL2 {
		return nil
	}
	softTTL := c.config.SoftTTL + c.jitter(key, c.config.SoftTTL/10)
	hardTTL := c.config.HardTTL + c.jitter(key, c.config.HardTTL/10)
	cached := envelope{Value: cloneBytes(value), SoftExpiresAt: now.Add(softTTL).UnixMilli(), HardExpiresAt: now.Add(hardTTL).UnixMilli()}
	raw, err := json.Marshal(cached)
	if err != nil {
		return err
	}
	opCtx, cancel := c.operationContext(ctx)
	defer cancel()
	return c.redis.Set(opCtx, c.redisKey(key), raw, hardTTL).Err()
}

func (c *Coordinator) setL1(key string, value []byte, now time.Time) {
	if c.config.EnableL1 {
		c.local.set(key, value, now.Add(c.config.L1TTL))
	}
}

func (c *Coordinator) refreshAsync(ctx context.Context, key string, loader func(context.Context) ([]byte, error)) {
	go func() {
		refreshCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, _, _ = c.group.Do("refresh:"+key, func() (any, error) {
			value, err := loader(refreshCtx)
			if err != nil {
				refreshTotal.WithLabelValues("error").Inc()
				return nil, err
			}
			err = c.store(refreshCtx, key, value)
			if err != nil {
				refreshTotal.WithLabelValues("error").Inc()
			} else {
				refreshTotal.WithLabelValues("success").Inc()
			}
			return nil, err
		})
	}()
}

func (c *Coordinator) redisKey(key string) string { return c.config.Prefix + ":" + key }

func (c *Coordinator) operationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, c.config.OperationTimeout)
}

func (c *Coordinator) jitter(key string, max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(key))
	return time.Duration(uint64(max) * uint64(hash.Sum32()%100) / 100)
}

func (c *Coordinator) String() string {
	return fmt.Sprintf("cache(prefix=%s,l1=%t,l2=%t)", c.config.Prefix, c.config.EnableL1, c.config.EnableL2)
}
