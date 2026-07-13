package repository

import (
	"fmt"
	"hash/crc32"
)

func NormalizeShardCount(configured int) int {
	if configured <= 0 {
		return 1
	}
	return configured
}

func StockShardKeys(productID int64, shardCount int) []string {
	shardCount = NormalizeShardCount(shardCount)
	keys := make([]string, 0, shardCount)
	for i := 0; i < shardCount; i++ {
		keys = append(keys, fmt.Sprintf("stock:%d:%d", productID, i))
	}
	return keys
}

func StockShardStartIndex(orderID string, shardCount int) int {
	shardCount = NormalizeShardCount(shardCount)
	if shardCount <= 1 {
		return 0
	}
	sum := crc32.ChecksumIEEE([]byte(orderID))
	return int(sum % uint32(shardCount))
}

func SplitStockAcrossShards(total int64, shardCount int) []int64 {
	shardCount = NormalizeShardCount(shardCount)
	if total < 0 {
		total = 0
	}
	values := make([]int64, shardCount)
	per := total / int64(shardCount)
	remain := total % int64(shardCount)
	for i := range values {
		values[i] = per
		if i == 0 {
			values[i] += remain
		}
	}
	return values
}
