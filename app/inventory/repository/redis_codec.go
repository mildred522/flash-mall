package repository

import (
	"context"
	"fmt"
	"strconv"

	"flash-mall/app/inventory/domain"
)

type redisReservation struct {
	ProductID  int64
	Quantity   int64
	ShardIndex int
}

func reservationKey(orderID string) string {
	return "inventory:reservation:" + orderID
}

func reservedStockKey(productID int64) string {
	return fmt.Sprintf("stock_reserved:%d", productID)
}

func uniquePositiveProductIDs(productIDs []int64) []int64 {
	seen := make(map[int64]struct{}, len(productIDs))
	result := make([]int64, 0, len(productIDs))
	for _, productID := range productIDs {
		if productID <= 0 {
			continue
		}
		if _, ok := seen[productID]; ok {
			continue
		}
		seen[productID] = struct{}{}
		result = append(result, productID)
	}
	return result
}

func orderStocksByProductIDs(productIDs []int64, stocks []domain.Stock) []domain.Stock {
	byID := make(map[int64]domain.Stock, len(stocks))
	for _, stock := range stocks {
		byID[stock.ProductID] = stock
	}
	ordered := make([]domain.Stock, 0, len(stocks))
	for _, productID := range productIDs {
		if stock, ok := byID[productID]; ok {
			ordered = append(ordered, stock)
		}
	}
	return ordered
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
	reservation := redisReservation{ProductID: parts[0], Quantity: parts[1]}
	if len(parts) > 2 {
		reservation.ShardIndex = int(parts[2])
	}
	return reservation, nil
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
