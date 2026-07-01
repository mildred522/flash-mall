package main

import (
	"database/sql"
	"log"
	"net"
	"os"
	"strconv"

	"flash-mall/app/inventory/kitex/kitex_gen/flashmall/inventory/inventoryservice"
	"flash-mall/app/inventory/repository"
	"flash-mall/app/inventory/service"

	"github.com/cloudwego/kitex/server"
	_ "github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	defaultListenOn        = "0.0.0.0:8093"
	defaultStockShardCount = 4
)

func main() {
	listenOn := envOrDefault("INVENTORY_LISTEN_ON", defaultListenOn)
	addr, err := net.ResolveTCPAddr("tcp", listenOn)
	if err != nil {
		log.Fatalf("resolve inventory listen addr %q: %v", listenOn, err)
	}

	shardCount := envIntOrDefault("INVENTORY_STOCK_SHARD_COUNT", defaultStockShardCount)
	finalDeductEnabled := envBoolOrDefault("INVENTORY_FINAL_DEDUCT_ENABLED", false)
	inventoryService := service.New(newRepository(shardCount, finalDeductEnabled), shardCount)
	svr := inventoryservice.NewServer(NewInventoryServiceImpl(inventoryService), server.WithServiceAddr(addr))

	if err := svr.Run(); err != nil {
		log.Println(err.Error())
	}
}

func newRepository(shardCount int, finalDeductEnabled bool) repository.StockRepository {
	redisHost := os.Getenv("INVENTORY_REDIS_HOST")
	if redisHost == "" {
		log.Println("INVENTORY_REDIS_HOST is empty; using in-memory inventory repository")
		return repository.NewMemoryStockRepository()
	}

	var db *sql.DB
	if dsn := os.Getenv("INVENTORY_DATASOURCE"); dsn != "" {
		opened, err := sql.Open("mysql", dsn)
		if err != nil {
			log.Fatalf("open inventory datasource: %v", err)
		}
		db = opened
	}

	rds := redis.MustNewRedis(redis.RedisConf{Host: redisHost, Type: redis.NodeType})
	return repository.NewRedisMySQLRepository(rds, db, shardCount).WithFinalDeductEnabled(finalDeductEnabled)
}

func envOrDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func envBoolOrDefault(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
