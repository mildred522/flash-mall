package main

import (
	"context"
	"testing"

	"flash-mall/app/inventory/repository"
	"flash-mall/app/inventory/service"

	"github.com/prometheus/client_golang/prometheus"
)

func TestRuntimeStateFromEnvironment(t *testing.T) {
	t.Setenv("INVENTORY_REDIS_HOST", "redis:6379")
	t.Setenv("INVENTORY_DATASOURCE", "user:pass@tcp(mysql:3306)/mall_product")
	t.Setenv("INVENTORY_RESERVATION_LEDGER_MODE", "shadow")

	state := runtimeStateFromEnvironment(8, true)
	if !state.FinalDeductEnabled || !state.RedisConfigured || !state.MySQLConfigured {
		t.Fatalf("unexpected runtime capabilities: %+v", state)
	}
	if state.ShardCount != 8 || state.ReservationLedgerMode != "shadow" {
		t.Fatalf("unexpected runtime configuration: %+v", state)
	}
}

func TestRuntimeStateRejectsUnknownLedgerMode(t *testing.T) {
	t.Setenv("INVENTORY_RESERVATION_LEDGER_MODE", "surprise")

	state := runtimeStateFromEnvironment(4, false)
	if state.ReservationLedgerMode != "off" {
		t.Fatalf("ReservationLedgerMode=%q, want off", state.ReservationLedgerMode)
	}
}

func TestTracingConfigFromEnvironment(t *testing.T) {
	t.Setenv("INVENTORY_TRACING_ENABLED", "true")
	t.Setenv("INVENTORY_TRACING_ENDPOINT", "http://jaeger:4318/v1/traces")
	t.Setenv("INVENTORY_TRACING_SAMPLE_RATIO", "0.75")

	cfg := tracingConfigFromEnvironment()
	if !cfg.Enabled || cfg.ServiceName != "inventory-kitex" || cfg.Exporter != "otlphttp" {
		t.Fatalf("unexpected tracing identity: %+v", cfg)
	}
	if cfg.Endpoint != "http://jaeger:4318/v1/traces" || cfg.SampleRatio != 0.75 {
		t.Fatalf("unexpected tracing transport: %+v", cfg)
	}
}

func TestRunReservationRecoveryOnce(t *testing.T) {
	svc := service.New(repository.NewMemoryStockRepository(), 4)
	if err := runReservationRecoveryOnce(context.Background(), svc, nil, 10); err != nil {
		t.Fatal(err)
	}
}

func TestRunReservationMetricsOnce(t *testing.T) {
	svc := service.New(repository.NewMemoryStockRepository(), 4)
	metrics := newInventoryMetrics(prometheus.NewRegistry())
	if err := runReservationMetricsOnce(context.Background(), svc, metrics); err != nil {
		t.Fatal(err)
	}
}
