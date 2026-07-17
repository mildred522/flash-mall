package ports

import "context"

type InventorySeedStatus int8

const (
	InventorySeedPending InventorySeedStatus = iota
	InventorySeedSucceeded
	InventorySeedFailed
)

type InventorySeedState struct {
	ProductID int64
	Status    InventorySeedStatus
	Attempts  int64
	LastError string
}

type InventoryStockSeeder interface {
	SeedStock(ctx context.Context, productID int64, total int64, shardCount int32, meta RequestMeta) error
}

type ProductInventoryInitializer interface {
	Initialize(ctx context.Context, productID int64, meta RequestMeta) (InventorySeedState, error)
}

type SnapshotRebuildRequest struct {
	ProductID int64
	Limit     int64
}

type ProductSnapshotStore interface {
	RebuildStock(context.Context, SnapshotRebuildRequest) (int64, error)
	RebuildCards(context.Context, SnapshotRebuildRequest) (int64, error)
}
