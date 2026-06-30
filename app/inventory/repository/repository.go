package repository

import (
	"context"

	"flash-mall/app/inventory/domain"
)

type StockRepository interface {
	GetStock(ctx context.Context, productID int64) (domain.Stock, error)
	SeedStock(ctx context.Context, productID int64, total int64, shardCount int) error
	ReserveStock(ctx context.Context, orderID string, productID int64, quantity int64) error
	ConfirmDeduct(ctx context.Context, orderID string) error
	ReleaseStock(ctx context.Context, orderID string, reason string) error
	ReconcileStock(ctx context.Context, productID int64) (before domain.Stock, after domain.Stock, changed bool, err error)
}
