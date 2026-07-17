package repository

import (
	"context"

	"flash-mall/app/inventory/domain"
)

type StockRepository interface {
	CheckRuntime(ctx context.Context) error
	GetStock(ctx context.Context, productID int64) (domain.Stock, error)
	BatchGetStock(ctx context.Context, productIDs []int64) ([]domain.Stock, error)
	SeedStock(ctx context.Context, productID int64, total int64, shardCount int) error
	AdjustStock(ctx context.Context, productID int64, delta int64, bucketIdx int, meta domain.StockChangeMeta) (before domain.Stock, after domain.Stock, err error)
	ReserveStock(ctx context.Context, orderID string, productID int64, quantity int64, meta domain.StockChangeMeta) error
	ConfirmDeduct(ctx context.Context, orderID string, meta domain.StockChangeMeta) error
	ReleaseStock(ctx context.Context, orderID string, meta domain.StockChangeMeta) error
	ReleaseExpiredReservations(ctx context.Context, limit int, meta domain.StockChangeMeta) (int, error)
	ReservationStats(ctx context.Context) (domain.ReservationStats, error)
	ReconcileStock(ctx context.Context, productID int64, meta domain.StockChangeMeta) (before domain.Stock, after domain.Stock, changed bool, err error)
}
