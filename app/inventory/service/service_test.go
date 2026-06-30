package service

import (
	"context"
	"testing"

	"flash-mall/app/common/apperror"
	"flash-mall/app/inventory/repository"
)

func TestReserveConfirmAndRelease(t *testing.T) {
	svc := New(repository.NewMemoryStockRepository(), 4)
	ctx := context.Background()
	if err := svc.SeedStock(ctx, 100, 10, 0); err != nil {
		t.Fatalf("SeedStock error: %v", err)
	}
	if err := svc.ReserveStock(ctx, "order-1", 100, 3); err != nil {
		t.Fatalf("ReserveStock error: %v", err)
	}
	stock, err := svc.GetStock(ctx, 100)
	if err != nil {
		t.Fatalf("GetStock error: %v", err)
	}
	if stock.Available != 7 || stock.Reserved != 3 || stock.Total != 10 {
		t.Fatalf("unexpected reserved stock: %+v", stock)
	}
	if err := svc.ReleaseStock(ctx, "order-1", "cancel"); err != nil {
		t.Fatalf("ReleaseStock error: %v", err)
	}
	stock, _ = svc.GetStock(ctx, 100)
	if stock.Available != 10 || stock.Reserved != 0 || stock.Total != 10 {
		t.Fatalf("unexpected released stock: %+v", stock)
	}
	if err := svc.ReserveStock(ctx, "order-2", 100, 4); err != nil {
		t.Fatalf("ReserveStock second error: %v", err)
	}
	if err := svc.ConfirmDeduct(ctx, "order-2"); err != nil {
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
	err := svc.ReserveStock(ctx, "order-1", 100, 3)
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
	if err := svc.ReserveStock(ctx, "order-1", 100, 2); err != nil {
		t.Fatalf("ReserveStock error: %v", err)
	}
	if err := svc.ReserveStock(ctx, "order-1", 100, 2); err != nil {
		t.Fatalf("ReserveStock retry error: %v", err)
	}
	stock, _ := svc.GetStock(ctx, 100)
	if stock.Available != 3 || stock.Reserved != 2 {
		t.Fatalf("reserve retry should be idempotent, got %+v", stock)
	}
}
