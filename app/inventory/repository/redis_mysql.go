package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"flash-mall/app/common/apperror"
	"flash-mall/app/inventory/domain"
)

const (
	reservationTTLSeconds          = 24 * 60 * 60
	confirmedReservationTTLSeconds = 180 * 24 * 60 * 60
)

type RedisClient interface {
	EvalCtx(ctx context.Context, script string, keys []string, args ...any) (any, error)
}

type RedisMySQLRepository struct {
	redis      RedisClient
	db         *sql.DB
	shardCount int
}

func NewRedisMySQLRepository(redis RedisClient, db *sql.DB, shardCount int) *RedisMySQLRepository {
	return &RedisMySQLRepository{redis: redis, db: db, shardCount: NormalizeShardCount(shardCount)}
}

func (r *RedisMySQLRepository) GetStock(ctx context.Context, productID int64) (domain.Stock, error) {
	available, exists, err := r.redisAvailable(ctx, productID)
	if err != nil {
		return domain.Stock{}, apperror.Wrap(apperror.CodeInternal, "read redis stock failed", err)
	}
	if exists {
		return domain.Stock{ProductID: productID, Available: available, Total: available}, nil
	}
	if r.db == nil {
		return domain.Stock{}, domain.ErrStockNotFound
	}
	total, ok, err := r.mysqlBucketTotal(ctx, productID)
	if err != nil {
		return domain.Stock{}, err
	}
	if !ok {
		return domain.Stock{}, domain.ErrStockNotFound
	}
	return domain.Stock{ProductID: productID, Available: total, Total: total}, nil
}

func (r *RedisMySQLRepository) SeedStock(ctx context.Context, productID int64, total int64, shardCount int) error {
	if shardCount <= 0 {
		shardCount = r.shardCount
	}
	if err := r.seedRedis(ctx, productID, total, shardCount); err != nil {
		return err
	}
	if r.db != nil {
		return r.seedMySQLBuckets(ctx, productID, total, shardCount)
	}
	return nil
}

func (r *RedisMySQLRepository) ReserveStock(ctx context.Context, orderID string, productID int64, quantity int64) error {
	keys := append(StockShardKeys(productID, r.shardCount), reservationKey(orderID))
	ret, err := evalInt64(ctx, r.redis, reserveStockLuaScript, keys, quantity, reservationTTLSeconds, r.shardCount, StockShardStartIndex(orderID, r.shardCount), productID, orderID)
	if err != nil {
		return apperror.Wrap(apperror.CodeStockReserveFailed, "reserve stock failed", err)
	}
	switch ret {
	case 1:
		return nil
	case -1:
		return domain.ErrStockNotFound
	case -2:
		return domain.ErrStockInsufficient
	default:
		return apperror.New(apperror.CodeStockReserveFailed, fmt.Sprintf("unexpected reserve result %d", ret))
	}
}

func (r *RedisMySQLRepository) ConfirmDeduct(ctx context.Context, orderID string) error {
	_, err := evalInt64(ctx, r.redis, confirmDeductLuaScript, []string{reservationKey(orderID)}, confirmedReservationTTLSeconds)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "confirm stock deduction failed", err)
	}
	return nil
}

func (r *RedisMySQLRepository) ReleaseStock(ctx context.Context, orderID string, reason string) error {
	_ = reason
	// The reservation stores a 1-based shard index, so all stock shard keys are needed for restore.
	reservation, err := evalReservation(ctx, r.redis, getReservationLuaScript, []string{reservationKey(orderID)})
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "read stock reservation failed", err)
	}
	if reservation.ProductID <= 0 || reservation.Quantity <= 0 {
		return nil
	}
	keys := append(StockShardKeys(reservation.ProductID, r.shardCount), reservationKey(orderID))
	_, err = evalInt64(ctx, r.redis, releaseStockLuaScript, keys, reservationTTLSeconds, r.shardCount)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "release stock failed", err)
	}
	return nil
}

func (r *RedisMySQLRepository) ReconcileStock(ctx context.Context, productID int64) (domain.Stock, domain.Stock, bool, error) {
	before, err := r.GetStock(ctx, productID)
	if err != nil && apperror.CodeOf(err) != apperror.CodeStockNotFound {
		return domain.Stock{}, domain.Stock{}, false, err
	}
	if r.db == nil {
		return before, before, false, nil
	}
	total, ok, err := r.mysqlBucketTotal(ctx, productID)
	if err != nil {
		return domain.Stock{}, domain.Stock{}, false, err
	}
	if !ok {
		return before, before, false, domain.ErrStockNotFound
	}
	after := domain.Stock{ProductID: productID, Available: total, Total: total}
	changed := before.Available != after.Available
	if changed {
		if err := r.seedRedis(ctx, productID, total, r.shardCount); err != nil {
			return before, after, false, err
		}
	}
	return before, after, changed, nil
}

func (r *RedisMySQLRepository) redisAvailable(ctx context.Context, productID int64) (int64, bool, error) {
	keys := StockShardKeys(productID, r.shardCount)
	val, err := r.redis.EvalCtx(ctx, sumStockLuaScript, keys)
	if err != nil {
		return 0, false, err
	}
	parts, err := toInt64Slice(val)
	if err != nil {
		return 0, false, err
	}
	if len(parts) < 2 {
		return 0, false, fmt.Errorf("unexpected redis stock sum result %v", val)
	}
	return parts[1], parts[0] == 1, nil
}

func (r *RedisMySQLRepository) seedRedis(ctx context.Context, productID int64, total int64, shardCount int) error {
	keys := StockShardKeys(productID, shardCount)
	values := SplitStockAcrossShards(total, shardCount)
	args := make([]any, 0, len(values))
	for _, value := range values {
		args = append(args, value)
	}
	if _, err := r.redis.EvalCtx(ctx, seedStockLuaScript, keys, args...); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "seed redis stock failed", err)
	}
	return nil
}

func (r *RedisMySQLRepository) mysqlBucketTotal(ctx context.Context, productID int64) (int64, bool, error) {
	if r.db == nil {
		return 0, false, nil
	}
	var total int64
	var count int64
	if err := r.db.QueryRowContext(ctx, "SELECT COALESCE(SUM(stock), 0), COUNT(*) FROM product_stock_bucket WHERE product_id = ?", productID).Scan(&total, &count); err != nil {
		return 0, false, apperror.Wrap(apperror.CodeInternal, "read mysql stock bucket failed", err)
	}
	return total, count > 0, nil
}

func (r *RedisMySQLRepository) seedMySQLBuckets(ctx context.Context, productID int64, total int64, shardCount int) error {
	values := SplitStockAcrossShards(total, shardCount)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "begin stock seed transaction failed", err)
	}
	defer tx.Rollback()
	for idx, value := range values {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO product_stock_bucket (product_id, bucket_idx, stock, version) VALUES (?, ?, ?, 0) ON DUPLICATE KEY UPDATE stock = VALUES(stock), version = version + 1",
			productID, idx, value,
		); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "seed mysql stock bucket failed", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "commit stock seed transaction failed", err)
	}
	return nil
}

type redisReservation struct {
	ProductID int64
	Quantity  int64
}

func reservationKey(orderID string) string {
	return "inventory:reservation:" + orderID
}

func evalInt64(ctx context.Context, redis RedisClient, script string, keys []string, args ...any) (int64, error) {
	val, err := redis.EvalCtx(ctx, script, keys, args...)
	if err != nil {
		return 0, err
	}
	return toInt64(val)
}

func evalReservation(ctx context.Context, redis RedisClient, script string, keys []string, args ...any) (redisReservation, error) {
	val, err := redis.EvalCtx(ctx, script, keys, args...)
	if err != nil {
		return redisReservation{}, err
	}
	parts, err := toInt64Slice(val)
	if err != nil {
		return redisReservation{}, err
	}
	if len(parts) < 2 {
		return redisReservation{}, fmt.Errorf("unexpected reservation result %v", val)
	}
	return redisReservation{ProductID: parts[0], Quantity: parts[1]}, nil
}

func toInt64(val any) (int64, error) {
	switch typed := val.(type) {
	case int64:
		return typed, nil
	case int:
		return int64(typed), nil
	case string:
		return strconv.ParseInt(typed, 10, 64)
	case []byte:
		return strconv.ParseInt(string(typed), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected redis eval result type %T", val)
	}
}

func toInt64Slice(val any) ([]int64, error) {
	switch typed := val.(type) {
	case []any:
		out := make([]int64, 0, len(typed))
		for _, item := range typed {
			v, err := toInt64(item)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil

	default:
		return nil, fmt.Errorf("unexpected redis eval array type %T", val)
	}
}

const seedStockLuaScript = `
for i = 1, #KEYS do
  redis.call("set", KEYS[i], ARGV[i])
end
return 1
`

const sumStockLuaScript = `
local exists = 0
local total = 0
for i = 1, #KEYS do
  local stock = redis.call("get", KEYS[i])
  if stock then
    exists = 1
    total = total + tonumber(stock)
  end
end
return {exists, total}
`

const reserveStockLuaScript = `
local shardCount = tonumber(ARGV[3])
local reservationKey = KEYS[shardCount + 1]
local status = redis.call("hget", reservationKey, "status")
if status and status ~= "released" then
  return 1
end
local amount = tonumber(ARGV[1])
local ttl = tonumber(ARGV[2])
local start = tonumber(ARGV[4])
local productID = ARGV[5]
local orderID = ARGV[6]
local hasStockKey = false
for i = 0, shardCount - 1 do
  local idx = ((start + i) % shardCount) + 1
  local stockKey = KEYS[idx]
  local stock = redis.call("get", stockKey)
  if stock then
    hasStockKey = true
    stock = tonumber(stock)
    if stock >= amount then
      redis.call("decrby", stockKey, amount)
      redis.call("hset", reservationKey, "status", "reserved", "product_id", productID, "quantity", amount, "shard_index", idx, "order_id", orderID)
      if ttl and ttl > 0 then
        redis.call("expire", reservationKey, ttl)
      end
      return 1
    end
  end
end
if hasStockKey == false then
  return -1
end
return -2
`

const confirmDeductLuaScript = `
local reservationKey = KEYS[1]
local status = redis.call("hget", reservationKey, "status")
if not status then
  return 1
end
if status == "reserved" then
  redis.call("hset", reservationKey, "status", "confirmed")
end
local ttl = tonumber(ARGV[1])
if ttl and ttl > 0 then
  redis.call("expire", reservationKey, ttl)
end
return 1
`

const getReservationLuaScript = `
local reservationKey = KEYS[1]
local productID = redis.call("hget", reservationKey, "product_id")
local quantity = redis.call("hget", reservationKey, "quantity")
if not productID or not quantity then
  return {0, 0}
end
return {tonumber(productID), tonumber(quantity)}
`

const releaseStockLuaScript = `
local shardCount = tonumber(ARGV[2])
local reservationKey = KEYS[shardCount + 1]
local status = redis.call("hget", reservationKey, "status")
if not status or status == "released" then
  return 1
end
local quantity = tonumber(redis.call("hget", reservationKey, "quantity"))
local shardIndex = tonumber(redis.call("hget", reservationKey, "shard_index"))
local stockKey = KEYS[shardIndex]
if stockKey and quantity and quantity > 0 then
  redis.call("incrby", stockKey, quantity)
end
redis.call("hset", reservationKey, "status", "released")
local ttl = tonumber(ARGV[1])
if ttl and ttl > 0 then
  redis.call("expire", reservationKey, ttl)
end
return 1
`
