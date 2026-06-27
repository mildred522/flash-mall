package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

func main() {
	var productId int64
	var stock int64
	var shards int
	flag.Int64Var(&productId, "product", 100, "product id")
	flag.Int64Var(&stock, "stock", 10000, "total stock")
	flag.IntVar(&shards, "shards", 4, "stock shard count")
	flag.Parse()

	rds := redis.MustNewRedis(redis.RedisConf{
		Host: "127.0.0.1:6379",
		Type: redis.NodeType,
	})

	if shards <= 0 {
		shards = 1
	}
	perShard := stock / int64(shards)
	remain := stock % int64(shards)
	for i := 0; i < shards; i++ {
		value := perShard
		if i == 0 {
			value += remain
		}
		key := fmt.Sprintf("stock:%d:%d", productId, i)
		// CHG 2026-02-07: 变更=按分片写入库存; 之前=单 key; 原因=分散热点提升并发。
		err := rds.SetCtx(context.Background(), key, fmt.Sprintf("%d", value))
		if err != nil {
			fmt.Printf("Failed to set stock shard: key=%s err=%v\n", key, err)
			return
		}
	}
	fmt.Printf("Stock seeded: product=%d total=%d shards=%d\n", productId, stock, shards)
}
