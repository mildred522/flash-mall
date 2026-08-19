package service

import (
	"context"

	"flash-mall/app/inventory/domain"
	"flash-mall/app/inventory/repository"
)

type Service struct {
	repo              repository.StockRepository
	defaultShardCount int
	runtimeState      RuntimeState
}

func New(repo repository.StockRepository, defaultShardCount int) *Service {
	shardCount := repository.NormalizeShardCount(defaultShardCount)
	return &Service{
		repo:              repo,
		defaultShardCount: shardCount,
		runtimeState: RuntimeState{
			ShardCount:            int64(shardCount),
			ReservationLedgerMode: "off",
		},
	}
}

type RuntimeState struct {
	FinalDeductEnabled    bool
	RedisConfigured       bool
	MySQLConfigured       bool
	ShardCount            int64
	ReservationLedgerMode string
}

func (s *Service) GetRuntimeState(_ context.Context) RuntimeState {
	return s.runtimeState
}

func (s *Service) WithRuntimeState(state RuntimeState) *Service {
	if state.ShardCount <= 0 {
		state.ShardCount = int64(s.defaultShardCount)
	}
	if state.ReservationLedgerMode == "" {
		state.ReservationLedgerMode = "off"
	}
	s.runtimeState = state
	return s
}

func (s *Service) CheckRuntime(ctx context.Context) error {
	return s.repo.CheckRuntime(ctx)
}

func (s *Service) RecoverRuntime(ctx context.Context) (domain.RuntimeRecoveryReport, error) {
	return s.repo.RecoverRuntime(ctx)
}

func (s *Service) GetStock(ctx context.Context, productID int64) (domain.Stock, error) {
	if productID <= 0 {
		return domain.Stock{}, domain.ErrProductIDRequired
	}
	return s.repo.GetStock(ctx, productID)
}

func (s *Service) BatchGetStock(ctx context.Context, productIDs []int64) ([]domain.Stock, error) {
	if len(productIDs) == 0 {
		return []domain.Stock{}, nil
	}
	for _, productID := range productIDs {
		if productID <= 0 {
			return nil, domain.ErrProductIDRequired
		}
	}
	return s.repo.BatchGetStock(ctx, productIDs)
}

func (s *Service) SeedStock(ctx context.Context, productID int64, total int64, shardCount int) error {
	if productID <= 0 {
		return domain.ErrProductIDRequired
	}
	if total < 0 {
		return domain.ErrQuantityInvalid
	}
	if shardCount <= 0 {
		shardCount = s.defaultShardCount
	}
	return s.repo.SeedStock(ctx, productID, total, shardCount)
}

func (s *Service) AdjustStock(ctx context.Context, productID int64, delta int64, bucketIdx int, meta domain.StockChangeMeta) (before domain.Stock, after domain.Stock, err error) {
	if productID <= 0 {
		return domain.Stock{}, domain.Stock{}, domain.ErrProductIDRequired
	}
	if delta == 0 {
		return domain.Stock{}, domain.Stock{}, domain.ErrQuantityInvalid
	}
	if bucketIdx < 0 {
		return domain.Stock{}, domain.Stock{}, domain.ErrQuantityInvalid
	}
	return s.repo.AdjustStock(ctx, productID, delta, bucketIdx, meta)
}

func (s *Service) ReserveStock(ctx context.Context, orderID string, productID int64, quantity int64, meta domain.StockChangeMeta) error {
	if orderID == "" {
		return domain.ErrOrderIDRequired
	}
	if productID <= 0 {
		return domain.ErrProductIDRequired
	}
	if quantity <= 0 {
		return domain.ErrQuantityInvalid
	}
	meta.OrderID = orderID
	return s.repo.ReserveStock(ctx, orderID, productID, quantity, meta)
}

func (s *Service) ConfirmDeduct(ctx context.Context, orderID string, meta domain.StockChangeMeta) error {
	if orderID == "" {
		return domain.ErrOrderIDRequired
	}
	meta.OrderID = orderID
	return s.repo.ConfirmDeduct(ctx, orderID, meta)
}

func (s *Service) ReleaseStock(ctx context.Context, orderID string, reason string, meta domain.StockChangeMeta) error {
	if orderID == "" {
		return domain.ErrOrderIDRequired
	}
	meta.OrderID = orderID
	if meta.Reason == "" {
		meta.Reason = reason
	}
	return s.repo.ReleaseStock(ctx, orderID, meta)
}

func (s *Service) ReleaseExpiredReservations(ctx context.Context, limit int, meta domain.StockChangeMeta) (int, error) {
	return s.repo.ReleaseExpiredReservations(ctx, limit, meta)
}

func (s *Service) ReservationStats(ctx context.Context) (domain.ReservationStats, error) {
	return s.repo.ReservationStats(ctx)
}

func (s *Service) ReconcileStock(ctx context.Context, productID int64, meta domain.StockChangeMeta) (before domain.Stock, after domain.Stock, changed bool, err error) {
	if productID <= 0 {
		return domain.Stock{}, domain.Stock{}, false, domain.ErrProductIDRequired
	}
	return s.repo.ReconcileStock(ctx, productID, meta)
}
