package main

import (
	"context"
	"database/sql"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"flash-mall/app/inventory/domain"
	"flash-mall/app/inventory/kitex/kitex_gen/flashmall/inventory/inventoryservice"
	"flash-mall/app/inventory/repository"
	"flash-mall/app/inventory/service"

	"github.com/cloudwego/kitex/server"
	_ "github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	defaultListenOn                   = "0.0.0.0:8093"
	defaultMetricsListenOn            = "0.0.0.0:9093"
	defaultStockShardCount            = 4
	defaultRecoveryInterval           = time.Minute
	defaultRecoveryBatchSize          = 100
	defaultReservationMetricsInterval = 15 * time.Second
)

func main() {
	listenOn := envOrDefault("INVENTORY_LISTEN_ON", defaultListenOn)
	addr, err := net.ResolveTCPAddr("tcp", listenOn)
	if err != nil {
		log.Fatalf("resolve inventory listen addr %q: %v", listenOn, err)
	}

	shardCount := envIntOrDefault("INVENTORY_STOCK_SHARD_COUNT", defaultStockShardCount)
	finalDeductEnabled := envBoolOrDefault("INVENTORY_FINAL_DEDUCT_ENABLED", false)
	registry := prometheus.NewRegistry()
	metrics := newInventoryMetrics(registry)
	startMetricsServer(envOrDefault("INVENTORY_METRICS_LISTEN_ON", defaultMetricsListenOn), registry)
	log.Printf("inventory-kitex starting: listen_on=%s shard_count=%d final_deduct_enabled=%t", listenOn, shardCount, finalDeductEnabled)
	runtimeState := runtimeStateFromEnvironment(shardCount, finalDeductEnabled)
	inventoryService := service.New(newRepository(shardCount, finalDeductEnabled, runtimeState.ReservationLedgerMode), shardCount).
		WithRuntimeState(runtimeState)
	if runtimeState.RedisConfigured {
		startReservationRecovery(inventoryService, metrics)
		startReservationMetricsSampler(inventoryService, metrics)
	}
	svr := inventoryservice.NewServer(NewInventoryServiceImpl(inventoryService, metrics), server.WithServiceAddr(addr))

	if err := svr.Run(); err != nil {
		log.Println(err.Error())
	}
}

func startReservationMetricsSampler(svc *service.Service, metrics *inventoryMetrics) {
	interval := envDurationOrDefault("INVENTORY_RESERVATION_METRICS_INTERVAL", defaultReservationMetricsInterval)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if err := runReservationMetricsOnce(context.Background(), svc, metrics); err != nil {
				log.Printf("inventory reservation metrics refresh failed: %v", err)
			}
			<-ticker.C
		}
	}()
}

func runReservationMetricsOnce(ctx context.Context, svc *service.Service, metrics *inventoryMetrics) error {
	stats, err := svc.ReservationStats(ctx)
	if err != nil {
		return err
	}
	metrics.refreshReservationStats(stats)
	return nil
}

func startReservationRecovery(svc *service.Service, metrics *inventoryMetrics) {
	interval := envDurationOrDefault("INVENTORY_RESERVATION_RECOVERY_INTERVAL", defaultRecoveryInterval)
	batchSize := envIntOrDefault("INVENTORY_RESERVATION_RECOVERY_BATCH_SIZE", defaultRecoveryBatchSize)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if err := runReservationRecoveryOnce(context.Background(), svc, metrics, batchSize); err != nil {
				log.Printf("inventory reservation recovery failed: %v", err)
			}
			<-ticker.C
		}
	}()
}

func runReservationRecoveryOnce(ctx context.Context, svc *service.Service, metrics *inventoryMetrics, batchSize int) error {
	processed, err := svc.ReleaseExpiredReservations(ctx, batchSize, domain.StockChangeMeta{Reason: "reservation timeout"})
	if metrics != nil {
		if err != nil {
			// Error counts represent failed recovery runs even when no item was
			// completed, so operational failures cannot remain invisible at zero.
			metrics.reservationRecovery.WithLabelValues("error").Inc()
		} else {
			metrics.reservationRecovery.WithLabelValues("success").Add(float64(processed))
		}
	}
	return err
}

func runtimeStateFromEnvironment(shardCount int, finalDeductEnabled bool) service.RuntimeState {
	ledgerMode := strings.ToLower(strings.TrimSpace(os.Getenv("INVENTORY_RESERVATION_LEDGER_MODE")))
	switch ledgerMode {
	case "shadow", "enforce":
	default:
		ledgerMode = "off"
	}
	return service.RuntimeState{
		FinalDeductEnabled:    finalDeductEnabled,
		RedisConfigured:       strings.TrimSpace(os.Getenv("INVENTORY_REDIS_HOST")) != "",
		MySQLConfigured:       strings.TrimSpace(os.Getenv("INVENTORY_DATASOURCE")) != "",
		ShardCount:            int64(repository.NormalizeShardCount(shardCount)),
		ReservationLedgerMode: ledgerMode,
	}
}

func startMetricsServer(listenOn string, registry *prometheus.Registry) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	go func() {
		if err := http.ListenAndServe(listenOn, mux); err != nil {
			log.Printf("inventory metrics server stopped: %v", err)
		}
	}()
}

func newRepository(shardCount int, finalDeductEnabled bool, ledgerMode string) repository.StockRepository {
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
	if finalDeductEnabled && db == nil {
		log.Fatalf("INVENTORY_FINAL_DEDUCT_ENABLED=true requires INVENTORY_DATASOURCE")
	}

	rds := redis.MustNewRedis(redis.RedisConf{Host: redisHost, Type: redis.NodeType})
	return repository.NewRedisMySQLRepository(rds, db, shardCount).
		WithFinalDeductEnabled(finalDeductEnabled).
		WithReservationLedgerMode(ledgerMode)
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

func envDurationOrDefault(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
