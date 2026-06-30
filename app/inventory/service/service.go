package service

import (
	"context"

	"flash-mall/app/inventory/domain"
	"flash-mall/app/inventory/repository"
)

type Service struct {
	repo              repository.StockRepository
	defaultShardCount int
}

func New(repo repository.StockRepository, defaultShardCount int) *Service {
	return &Service{repo: repo, defaultShardCount: repository.NormalizeShardCount(defaultShardCount)}
}

func (s *Service) GetStock(ctx context.Context, productID int64) (domain.Stock, error) {
	if productID <= 0 {
		return domain.Stock{}, domain.ErrProductIDRequired
	}
	return s.repo.GetStock(ctx, productID)
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

func (s *Service) ReserveStock(ctx context.Context, orderID string, productID int64, quantity int64) error {
	if orderID == "" {
		return domain.ErrOrderIDRequired
	}
	if productID <= 0 {
		return domain.ErrProductIDRequired
	}
	if quantity <= 0 {
		return domain.ErrQuantityInvalid
	}
	return s.repo.ReserveStock(ctx, orderID, productID, quantity)
}

func (s *Service) ConfirmDeduct(ctx context.Context, orderID string) error {
	if orderID == "" {
		return domain.ErrOrderIDRequired
	}
	return s.repo.ConfirmDeduct(ctx, orderID)
}

func (s *Service) ReleaseStock(ctx context.Context, orderID string, reason string) error {
	if orderID == "" {
		return domain.ErrOrderIDRequired
	}
	return s.repo.ReleaseStock(ctx, orderID, reason)
}

func (s *Service) ReconcileStock(ctx context.Context, productID int64) (before domain.Stock, after domain.Stock, changed bool, err error) {
	if productID <= 0 {
		return domain.Stock{}, domain.Stock{}, false, domain.ErrProductIDRequired
	}
	return s.repo.ReconcileStock(ctx, productID)
}
