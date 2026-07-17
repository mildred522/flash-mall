package ports

import "context"

type RequestMeta struct {
	RequestID  string
	TraceID    string
	UserID     int64
	MerchantID int64
	Role       string
}

type Stock struct {
	ProductID int64
	Available int64
	Reserved  int64
	Total     int64
}

type InventoryRuntimeState struct {
	FinalDeductEnabled       bool
	RedisConfigured          bool
	MySQLConfigured          bool
	ShardCount               int32
	ReservationLedgerMode    string
	ReservationLedgerEnabled bool
}

type InventoryService interface {
	GetStock(ctx context.Context, productID int64, meta RequestMeta) (Stock, error)
	BatchGetStock(ctx context.Context, productIDs []int64, meta RequestMeta) (map[int64]Stock, error)
	SeedStock(ctx context.Context, productID int64, total int64, shardCount int32, meta RequestMeta) error
	AdjustStock(ctx context.Context, productID int64, delta int64, bucketIdx int, reason string, meta RequestMeta) (Stock, error)
	ReserveStock(ctx context.Context, orderID string, productID int64, quantity int64, meta RequestMeta) error
	ConfirmDeduct(ctx context.Context, orderID string, meta RequestMeta) error
	ReleaseStock(ctx context.Context, orderID string, reason string, meta RequestMeta) error
	GetRuntimeState(ctx context.Context, meta RequestMeta) (InventoryRuntimeState, error)
}
