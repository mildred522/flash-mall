package config

import "github.com/zeromicro/go-zero/zrpc"

type Config struct {
	zrpc.RpcServerConf
	DataSource string
	// CHG 2026-02-24: 变更=新增库存分桶数量; 之前=单行库存; 原因=降低热点行冲突。
	StockBucketCount int
}
