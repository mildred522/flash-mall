package service

import (
	"context"
	"reflect"
	"testing"

	"flash-mall/app/common/apperror"
	"flash-mall/app/inventory/domain"
	"flash-mall/app/inventory/repository"
)

func TestRuntimeStateReportsSafeDefaults(t *testing.T) {
	svc := New(repository.NewMemoryStockRepository(), 8)
	method := reflect.ValueOf(svc).MethodByName("GetRuntimeState")
	if !method.IsValid() {
		t.Fatal("inventory service must expose GetRuntimeState")
	}
	results := method.Call([]reflect.Value{reflect.ValueOf(context.Background())})
	if len(results) != 1 {
		t.Fatalf("GetRuntimeState returned %d values", len(results))
	}
	state := results[0]
	assertRuntimeField(t, state, "ShardCount", int64(8))
	assertRuntimeField(t, state, "FinalDeductEnabled", false)
	assertRuntimeField(t, state, "RedisConfigured", false)
	assertRuntimeField(t, state, "MySQLConfigured", false)
	assertRuntimeField(t, state, "ReservationLedgerMode", "off")
}

func TestRuntimeStateReportsConfiguredCapabilities(t *testing.T) {
	svc := New(repository.NewMemoryStockRepository(), 8)
	configure := reflect.ValueOf(svc).MethodByName("WithRuntimeState")
	if !configure.IsValid() {
		t.Fatal("inventory service must allow runtime state configuration")
	}
	configured := RuntimeState{
		FinalDeductEnabled:    true,
		RedisConfigured:       true,
		MySQLConfigured:       true,
		ReservationLedgerMode: "shadow",
	}
	configure.Call([]reflect.Value{reflect.ValueOf(configured)})

	state := svc.GetRuntimeState(context.Background())
	if !state.FinalDeductEnabled || !state.RedisConfigured || !state.MySQLConfigured {
		t.Fatalf("runtime capabilities were not preserved: %+v", state)
	}
	if state.ShardCount != 8 {
		t.Fatalf("ShardCount=%d, want configured service default 8", state.ShardCount)
	}
	if state.ReservationLedgerMode != "shadow" {
		t.Fatalf("ReservationLedgerMode=%q, want shadow", state.ReservationLedgerMode)
	}
}

func assertRuntimeField(t *testing.T, state reflect.Value, name string, want any) {
	t.Helper()
	field := state.FieldByName(name)
	if !field.IsValid() {
		t.Fatalf("runtime state field %s is missing", name)
	}
	if got := field.Interface(); !reflect.DeepEqual(got, want) {
		t.Fatalf("runtime state %s=%v, want %v", name, got, want)
	}
}

func TestReserveConfirmAndRelease(t *testing.T) {
	svc := New(repository.NewMemoryStockRepository(), 4)
	ctx := context.Background()
	if err := svc.SeedStock(ctx, 100, 10, 0); err != nil {
		t.Fatalf("SeedStock error: %v", err)
	}
	if err := svc.ReserveStock(ctx, "order-1", 100, 3, domain.StockChangeMeta{}); err != nil {
		t.Fatalf("ReserveStock error: %v", err)
	}
	stock, err := svc.GetStock(ctx, 100)
	if err != nil {
		t.Fatalf("GetStock error: %v", err)
	}
	if stock.Available != 7 || stock.Reserved != 3 || stock.Total != 10 {
		t.Fatalf("unexpected reserved stock: %+v", stock)
	}
	if err := svc.ReleaseStock(ctx, "order-1", "cancel", domain.StockChangeMeta{}); err != nil {
		t.Fatalf("ReleaseStock error: %v", err)
	}
	stock, _ = svc.GetStock(ctx, 100)
	if stock.Available != 10 || stock.Reserved != 0 || stock.Total != 10 {
		t.Fatalf("unexpected released stock: %+v", stock)
	}
	if err := svc.ReserveStock(ctx, "order-2", 100, 4, domain.StockChangeMeta{}); err != nil {
		t.Fatalf("ReserveStock second error: %v", err)
	}
	if err := svc.ConfirmDeduct(ctx, "order-2", domain.StockChangeMeta{}); err != nil {
		t.Fatalf("ConfirmDeduct error: %v", err)
	}
	stock, _ = svc.GetStock(ctx, 100)
	if stock.Available != 6 || stock.Reserved != 0 || stock.Total != 6 {
		t.Fatalf("unexpected confirmed stock: %+v", stock)
	}
}

func TestReserveStockInsufficient(t *testing.T) {
	svc := New(repository.NewMemoryStockRepository(), 4)
	ctx := context.Background()
	if err := svc.SeedStock(ctx, 100, 2, 0); err != nil {
		t.Fatalf("SeedStock error: %v", err)
	}
	err := svc.ReserveStock(ctx, "order-1", 100, 3, domain.StockChangeMeta{})
	if apperror.CodeOf(err) != apperror.CodeStockInsufficient {
		t.Fatalf("CodeOf(err) = %s, want %s", apperror.CodeOf(err), apperror.CodeStockInsufficient)
	}
}

func TestReserveStockIdempotent(t *testing.T) {
	svc := New(repository.NewMemoryStockRepository(), 4)
	ctx := context.Background()
	if err := svc.SeedStock(ctx, 100, 5, 0); err != nil {
		t.Fatalf("SeedStock error: %v", err)
	}
	if err := svc.ReserveStock(ctx, "order-1", 100, 2, domain.StockChangeMeta{}); err != nil {
		t.Fatalf("ReserveStock error: %v", err)
	}
	if err := svc.ReserveStock(ctx, "order-1", 100, 2, domain.StockChangeMeta{}); err != nil {
		t.Fatalf("ReserveStock retry error: %v", err)
	}
	stock, _ := svc.GetStock(ctx, 100)
	if stock.Available != 3 || stock.Reserved != 2 {
		t.Fatalf("reserve retry should be idempotent, got %+v", stock)
	}
}

func TestReserveStockRejectsConflictingReplay(t *testing.T) {
	svc := New(repository.NewMemoryStockRepository(), 4)
	ctx := context.Background()
	if err := svc.SeedStock(ctx, 100, 5, 0); err != nil {
		t.Fatal(err)
	}
	if err := svc.SeedStock(ctx, 101, 5, 0); err != nil {
		t.Fatal(err)
	}
	if err := svc.ReserveStock(ctx, "order-conflict", 100, 2, domain.StockChangeMeta{}); err != nil {
		t.Fatal(err)
	}
	err := svc.ReserveStock(ctx, "order-conflict", 101, 3, domain.StockChangeMeta{})
	if apperror.CodeOf(err) != apperror.CodeConflict {
		t.Fatalf("CodeOf(err)=%s, want %s", apperror.CodeOf(err), apperror.CodeConflict)
	}
}

func TestReleaseExpiredReservationsDelegatesRecovery(t *testing.T) {
	svc := New(repository.NewMemoryStockRepository(), 4)
	processed, err := svc.ReleaseExpiredReservations(context.Background(), 10, domain.StockChangeMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if processed != 0 {
		t.Fatalf("processed=%d, want 0 for memory repository", processed)
	}
}
