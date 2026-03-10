package logic

import (
	"fmt"
	"hash/crc32"
)

// CHG 2026-02-07: 变更=新增库存分片工具; 之前=单 key 热点; 原因=分散高并发热点 key。
func stockShardCount(configured int) int {
	if configured <= 0 {
		return 1
	}
	return configured
}

func stockShardKeys(productId int64, shardCount int) []string {
	keys := make([]string, 0, shardCount)
	for i := 0; i < shardCount; i++ {
		keys = append(keys, fmt.Sprintf("stock:%d:%d", productId, i))
	}
	return keys
}

func stockShardStartIndex(orderId string, shardCount int) int {
	if shardCount <= 1 {
		return 0
	}
	sum := crc32.ChecksumIEEE([]byte(orderId))
	return int(sum % uint32(shardCount))
}
