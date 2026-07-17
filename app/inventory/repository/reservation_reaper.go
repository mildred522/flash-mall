package repository

import (
	"context"
	"fmt"
	"time"

	"flash-mall/app/common/apperror"
	"flash-mall/app/inventory/domain"
)

const (
	reservationProcessingIndexKey = "inventory:reservation:processing"
	reservationRetryCountKey      = "inventory:reservation:retry_count"
	reservationDeadLetterIndexKey = "inventory:reservation:dead_letter"
	reservationProcessingTimeout  = 5 * time.Minute
	reservationRetryDelay         = 30 * time.Second
	reservationMaxRecoveryRetries = 3
)

func (r *RedisMySQLRepository) ReleaseExpiredReservations(ctx context.Context, limit int, meta domain.StockChangeMeta) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	claimed, err := evalStringSlice(ctx, r.redis, claimExpiredReservationsLuaScript,
		[]string{reservationExpiryIndexKey, reservationProcessingIndexKey},
		time.Now().Unix(), limit, int64(reservationProcessingTimeout/time.Second),
	)
	if err != nil {
		return 0, apperror.Wrap(apperror.CodeInternal, "claim expired stock reservations failed", err)
	}
	processed := 0
	var firstErr error
	for _, orderID := range claimed {
		if err := r.ReleaseStock(ctx, orderID, meta); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			_, _ = evalInt64(ctx, r.redis, retryExpiredReservationLuaScript,
				[]string{reservationProcessingIndexKey, reservationExpiryIndexKey, reservationRetryCountKey, reservationDeadLetterIndexKey},
				orderID, time.Now().Unix(), int64(reservationRetryDelay/time.Second), reservationMaxRecoveryRetries,
			)
			continue
		}
		if _, err := evalInt64(ctx, r.redis, completeExpiredReservationLuaScript,
			[]string{reservationProcessingIndexKey, reservationRetryCountKey}, orderID,
		); err != nil && firstErr == nil {
			firstErr = apperror.Wrap(apperror.CodeInternal, "complete expired reservation recovery failed", err)
		}
		processed++
	}
	return processed, firstErr
}

func (r *RedisMySQLRepository) ReservationStats(ctx context.Context) (domain.ReservationStats, error) {
	value, err := r.redis.EvalCtx(ctx, reservationStatsLuaScript, []string{
		reservationExpiryIndexKey,
		reservationProcessingIndexKey,
		reservationDeadLetterIndexKey,
	}, time.Now().Unix())
	if err != nil {
		return domain.ReservationStats{}, apperror.Wrap(apperror.CodeInternal, "read reservation stats failed", err)
	}
	parts, err := toInt64Slice(value)
	if err != nil {
		return domain.ReservationStats{}, apperror.Wrap(apperror.CodeInternal, "decode reservation stats failed", err)
	}
	if len(parts) != 4 {
		return domain.ReservationStats{}, apperror.Wrap(apperror.CodeInternal, "decode reservation stats failed", fmt.Errorf("unexpected result length %d", len(parts)))
	}
	return domain.ReservationStats{Active: parts[0], Processing: parts[1], DeadLetter: parts[2], Expired: parts[3]}, nil
}

func evalStringSlice(ctx context.Context, redis RedisClient, script string, keys []string, args ...any) ([]string, error) {
	value, err := redis.EvalCtx(ctx, script, keys, args...)
	if err != nil {
		return nil, err
	}
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected redis string array type %T", value)
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		switch typed := item.(type) {
		case string:
			result = append(result, typed)
		case []byte:
			result = append(result, string(typed))
		default:
			return nil, fmt.Errorf("unexpected redis string result type %T", item)
		}
	}
	return result, nil
}

const claimExpiredReservationsLuaScript = `
local now = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local processingTimeout = tonumber(ARGV[3])
local abandoned = redis.call("zrangebyscore", KEYS[2], "-inf", now)
for _, orderID in ipairs(abandoned) do
  redis.call("zrem", KEYS[2], orderID)
  redis.call("zadd", KEYS[1], now, orderID)
end
local claimed = redis.call("zrangebyscore", KEYS[1], "-inf", now, "LIMIT", 0, limit)
for _, orderID in ipairs(claimed) do
  redis.call("zrem", KEYS[1], orderID)
  redis.call("zadd", KEYS[2], now + processingTimeout, orderID)
end
return claimed
`

const completeExpiredReservationLuaScript = `
redis.call("zrem", KEYS[1], ARGV[1])
redis.call("hdel", KEYS[2], ARGV[1])
return 1
`

const retryExpiredReservationLuaScript = `
local orderID = ARGV[1]
local now = tonumber(ARGV[2])
local retryDelay = tonumber(ARGV[3])
local maxRetries = tonumber(ARGV[4])
local attempts = redis.call("hincrby", KEYS[3], orderID, 1)
redis.call("zrem", KEYS[1], orderID)
if attempts >= maxRetries then
  redis.call("zadd", KEYS[4], now, orderID)
else
  redis.call("zadd", KEYS[2], now + retryDelay, orderID)
end
return attempts
`

const reservationStatsLuaScript = `
local now = tonumber(ARGV[1])
return {
  redis.call("zcard", KEYS[1]),
  redis.call("zcard", KEYS[2]),
  redis.call("zcard", KEYS[3]),
  redis.call("zcount", KEYS[1], "-inf", now)
}
`
