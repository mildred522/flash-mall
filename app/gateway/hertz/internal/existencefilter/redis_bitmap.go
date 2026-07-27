package existencefilter

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	redis "github.com/redis/go-redis/v9"
)

type RedisBitmap struct {
	config normalizedConfig
	client *redis.Client
	source IDSource
}

func NewRedisBitmap(config Config, client *redis.Client, source IDSource) *RedisBitmap {
	return &RedisBitmap{config: config.normalize(), client: client, source: source}
}

func (f *RedisBitmap) Check(ctx context.Context, productID int64) (result Result, err error) {
	defer func() { recordCheck(result, err) }()
	if f.client == nil || productID <= 0 {
		return ResultUnready, nil
	}
	generation, metadata, err := f.activeMetadata(ctx)
	if err != nil {
		if err == redis.Nil {
			return ResultUnready, nil
		}
		return ResultUnready, err
	}
	positions := bitPositions(productID, metadata.bitCount, metadata.hashCount)
	opCtx, cancel := f.operationContext(ctx)
	defer cancel()
	commands := make([]*redis.IntCmd, 0, len(positions))
	_, err = f.client.Pipelined(opCtx, func(pipe redis.Pipeliner) error {
		for _, position := range positions {
			commands = append(commands, pipe.GetBit(opCtx, f.bitsKey(generation), position))
		}
		return nil
	})
	if err != nil {
		return ResultUnready, err
	}
	for _, command := range commands {
		if command.Val() == 0 {
			return ResultAbsent, nil
		}
	}
	return ResultPossible, nil
}

func (f *RedisBitmap) Add(ctx context.Context, productID int64) (err error) {
	defer func() {
		result := "success"
		if err != nil {
			result = "error"
		}
		filterUpdates.WithLabelValues("product", result).Inc()
	}()
	if f.client == nil || productID <= 0 {
		return ErrUnready
	}
	opCtx, cancel := f.operationContext(ctx)
	rebuilding, lockErr := f.client.Exists(opCtx, f.lockKey()).Result()
	cancel()
	if lockErr != nil {
		return lockErr
	}
	if rebuilding > 0 {
		return ErrRebuildInProgress
	}
	generation, metadata, err := f.activeMetadata(ctx)
	if err != nil {
		if err == redis.Nil {
			return ErrUnready
		}
		return err
	}
	return f.addToGeneration(ctx, generation, metadata, productID)
}

func (f *RedisBitmap) Status(ctx context.Context) Status {
	status := Status{Enabled: f.client != nil, State: "unready"}
	if f.client == nil {
		return status
	}
	opCtx, cancel := f.operationContext(ctx)
	defer cancel()
	generation, err := f.client.Get(opCtx, f.activeKey()).Result()
	if err != nil {
		if err != redis.Nil {
			status.State = "degraded"
		}
		return status
	}
	values, err := f.client.HGetAll(opCtx, f.metaKey(generation)).Result()
	if err != nil {
		status.State = "degraded"
		return status
	}
	if values["state"] != "ready" || values["algorithm"] != hashAlgorithm {
		return status
	}
	items, err := strconv.ParseInt(values["count"], 10, 64)
	if err != nil {
		status.State = "degraded"
		return status
	}
	status.Ready = true
	status.State = "ready"
	status.Generation = generation
	status.Items = items
	return status
}

type generationMetadata struct {
	bitCount  uint64
	hashCount uint64
}

func (f *RedisBitmap) activeMetadata(ctx context.Context) (string, generationMetadata, error) {
	opCtx, cancel := f.operationContext(ctx)
	defer cancel()
	generation, err := f.client.Get(opCtx, f.activeKey()).Result()
	if err != nil {
		return "", generationMetadata{}, err
	}
	values, err := f.client.HGetAll(opCtx, f.metaKey(generation)).Result()
	if err != nil {
		return "", generationMetadata{}, err
	}
	if values["state"] != "ready" || values["algorithm"] != hashAlgorithm {
		return "", generationMetadata{}, redis.Nil
	}
	bitCount, bitErr := strconv.ParseUint(values["bit_count"], 10, 64)
	hashCount, hashErr := strconv.ParseUint(values["hash_count"], 10, 64)
	if bitErr != nil || hashErr != nil || bitCount == 0 || hashCount == 0 {
		return "", generationMetadata{}, redis.Nil
	}
	return generation, generationMetadata{bitCount: bitCount, hashCount: hashCount}, nil
}

func (f *RedisBitmap) addToGeneration(
	ctx context.Context,
	generation string,
	metadata generationMetadata,
	productID int64,
) error {
	opCtx, cancel := f.operationContext(ctx)
	defer cancel()
	positions := bitPositions(productID, metadata.bitCount, metadata.hashCount)
	_, err := f.client.Pipelined(opCtx, func(pipe redis.Pipeliner) error {
		for _, position := range positions {
			pipe.SetBit(opCtx, f.bitsKey(generation), position, 1)
		}
		return nil
	})
	return err
}

func (f *RedisBitmap) operationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, f.config.OperationTimeout)
}

func (f *RedisBitmap) activeKey() string { return f.config.Prefix + ":active" }
func (f *RedisBitmap) lockKey() string   { return f.config.Prefix + ":rebuild-lock" }
func (f *RedisBitmap) bitsKey(generation string) string {
	return f.config.Prefix + ":g:" + generation + ":bits"
}
func (f *RedisBitmap) metaKey(generation string) string {
	return f.config.Prefix + ":g:" + generation + ":meta"
}
func newGeneration() string { return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomToken()) }
func randomToken() string {
	var value [12]byte
	_, _ = rand.Read(value[:])
	return hex.EncodeToString(value[:])
}
