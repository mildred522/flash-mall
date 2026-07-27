package existencefilter

import (
	"context"
	"fmt"
	"time"

	redis "github.com/redis/go-redis/v9"
)

var (
	activateGenerationScript = redis.NewScript(`
if redis.call("HGET", KEYS[1], "state") ~= "ready" then
  return redis.error_reply("generation is not ready")
end
local previous = redis.call("GET", KEYS[2])
redis.call("SET", KEYS[2], ARGV[1])
return previous or ""
`)
	unlockScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`)
)

func (f *RedisBitmap) Rebuild(ctx context.Context) (err error) {
	startedAt := time.Now()
	var count int64
	defer func() {
		result := "success"
		if err != nil {
			result = "error"
		}
		filterRebuilds.WithLabelValues("product", result).Inc()
		filterRebuildDuration.WithLabelValues("product").Observe(time.Since(startedAt).Seconds())
		if err == nil {
			filterItems.WithLabelValues("product").Set(float64(count))
			filterReady.WithLabelValues("product").Set(1)
		}
	}()
	return f.rebuild(ctx, &count)
}

func (f *RedisBitmap) rebuild(ctx context.Context, rebuiltCount *int64) error {
	if f.client == nil || f.source == nil {
		return ErrUnready
	}
	token := randomToken()
	opCtx, cancel := f.operationContext(ctx)
	acquired, err := f.client.SetNX(opCtx, f.lockKey(), token, f.config.RebuildLockTTL).Result()
	cancel()
	if err != nil {
		return err
	}
	if !acquired {
		return ErrRebuildInProgress
	}
	defer func() {
		unlockCtx, unlockCancel := f.operationContext(context.WithoutCancel(ctx))
		defer unlockCancel()
		_, _ = unlockScript.Run(unlockCtx, f.client, []string{f.lockKey()}, token).Result()
	}()

	generation := newGeneration()
	metadata := generationMetadata{bitCount: f.config.bitCount, hashCount: f.config.hashCount}
	if err := f.writeBuildingMetadata(ctx, generation, metadata); err != nil {
		return err
	}
	count, err := f.loadGeneration(ctx, generation, metadata)
	if err != nil {
		f.deleteGeneration(context.WithoutCancel(ctx), generation)
		return err
	}
	if err := f.markReady(ctx, generation, count); err != nil {
		f.deleteGeneration(context.WithoutCancel(ctx), generation)
		return err
	}
	previous, err := f.activate(ctx, generation)
	if err != nil {
		f.deleteGeneration(context.WithoutCancel(ctx), generation)
		return err
	}
	if previous != "" && previous != generation {
		f.expireGeneration(context.WithoutCancel(ctx), previous)
	}
	*rebuiltCount = count
	return nil
}

func (f *RedisBitmap) writeBuildingMetadata(
	ctx context.Context,
	generation string,
	metadata generationMetadata,
) error {
	opCtx, cancel := f.operationContext(ctx)
	defer cancel()
	return f.client.HSet(opCtx, f.metaKey(generation), map[string]any{
		"state":      "building",
		"algorithm":  hashAlgorithm,
		"bit_count":  metadata.bitCount,
		"hash_count": metadata.hashCount,
	}).Err()
}

func (f *RedisBitmap) loadGeneration(
	ctx context.Context,
	generation string,
	metadata generationMetadata,
) (int64, error) {
	var afterID, count int64
	for {
		ids, err := f.source.ProductIDBatch(ctx, afterID, f.config.BatchSize)
		if err != nil {
			return count, err
		}
		if len(ids) == 0 {
			return count, nil
		}
		for _, id := range ids {
			if id <= afterID {
				return count, fmt.Errorf("product id source returned non-increasing id %d after %d", id, afterID)
			}
			afterID = id
		}
		if err := f.addBatch(ctx, generation, metadata, ids); err != nil {
			return count, err
		}
		count += int64(len(ids))
		if len(ids) < f.config.BatchSize {
			return count, nil
		}
	}
}

func (f *RedisBitmap) addBatch(
	ctx context.Context,
	generation string,
	metadata generationMetadata,
	ids []int64,
) error {
	batchTimeout := 5 * time.Second
	if f.config.OperationTimeout > batchTimeout {
		batchTimeout = f.config.OperationTimeout
	}
	opCtx, cancel := context.WithTimeout(ctx, batchTimeout)
	defer cancel()
	_, err := f.client.Pipelined(opCtx, func(pipe redis.Pipeliner) error {
		for _, id := range ids {
			for _, position := range bitPositions(id, metadata.bitCount, metadata.hashCount) {
				pipe.SetBit(opCtx, f.bitsKey(generation), position, 1)
			}
		}
		return nil
	})
	return err
}

func (f *RedisBitmap) markReady(ctx context.Context, generation string, count int64) error {
	opCtx, cancel := f.operationContext(ctx)
	defer cancel()
	return f.client.HSet(opCtx, f.metaKey(generation), map[string]any{
		"state":    "ready",
		"count":    count,
		"built_at": time.Now().Unix(),
	}).Err()
}

func (f *RedisBitmap) activate(ctx context.Context, generation string) (string, error) {
	opCtx, cancel := f.operationContext(ctx)
	defer cancel()
	value, err := activateGenerationScript.Run(
		opCtx, f.client, []string{f.metaKey(generation), f.activeKey()}, generation,
	).Result()
	if err != nil {
		return "", err
	}
	return fmt.Sprint(value), nil
}

func (f *RedisBitmap) expireGeneration(ctx context.Context, generation string) {
	opCtx, cancel := f.operationContext(ctx)
	defer cancel()
	_, _ = f.client.Pipelined(opCtx, func(pipe redis.Pipeliner) error {
		pipe.Expire(opCtx, f.bitsKey(generation), f.config.OldGenerationTTL)
		pipe.Expire(opCtx, f.metaKey(generation), f.config.OldGenerationTTL)
		return nil
	})
}

func (f *RedisBitmap) deleteGeneration(ctx context.Context, generation string) {
	opCtx, cancel := f.operationContext(ctx)
	defer cancel()
	_ = f.client.Del(opCtx, f.bitsKey(generation), f.metaKey(generation)).Err()
}
