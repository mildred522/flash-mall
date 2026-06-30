package repository

import (
	"context"
	"sync"

	"flash-mall/app/inventory/domain"
)

type MemoryStockRepository struct {
	mu           sync.Mutex
	stocks       map[int64]domain.Stock
	reservations map[string]domain.Reservation
}

func NewMemoryStockRepository() *MemoryStockRepository {
	return &MemoryStockRepository{
		stocks:       map[int64]domain.Stock{},
		reservations: map[string]domain.Reservation{},
	}
}

func (r *MemoryStockRepository) GetStock(ctx context.Context, productID int64) (domain.Stock, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	stock, ok := r.stocks[productID]
	if !ok {
		return domain.Stock{}, domain.ErrStockNotFound
	}
	return stock, nil
}

func (r *MemoryStockRepository) SeedStock(ctx context.Context, productID int64, total int64, shardCount int) error {
	_ = ctx
	_ = shardCount
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stocks[productID] = domain.NewStock(productID, total)
	return nil
}

func (r *MemoryStockRepository) ReserveStock(ctx context.Context, orderID string, productID int64, quantity int64) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.reservations[orderID]; ok {
		if existing.Status == domain.ReservationReleased {
			return nil
		}
		return nil
	}
	stock, ok := r.stocks[productID]
	if !ok {
		return domain.ErrStockNotFound
	}
	if !stock.CanReserve(quantity) {
		return domain.ErrStockInsufficient
	}
	stock.Available -= quantity
	stock.Reserved += quantity
	r.stocks[productID] = stock
	r.reservations[orderID] = domain.Reservation{OrderID: orderID, ProductID: productID, Quantity: quantity, Status: domain.ReservationReserved}
	return nil
}

func (r *MemoryStockRepository) ConfirmDeduct(ctx context.Context, orderID string) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	reservation, ok := r.reservations[orderID]
	if !ok || reservation.Status == domain.ReservationConfirmed {
		return nil
	}
	if reservation.Status == domain.ReservationReleased {
		return nil
	}
	stock := r.stocks[reservation.ProductID]
	stock.Reserved -= reservation.Quantity
	stock.Total -= reservation.Quantity
	if stock.Reserved < 0 {
		stock.Reserved = 0
	}
	if stock.Total < stock.Available+stock.Reserved {
		stock.Total = stock.Available + stock.Reserved
	}
	r.stocks[reservation.ProductID] = stock
	reservation.Status = domain.ReservationConfirmed
	r.reservations[orderID] = reservation
	return nil
}

func (r *MemoryStockRepository) ReleaseStock(ctx context.Context, orderID string, reason string) error {
	_ = ctx
	_ = reason
	r.mu.Lock()
	defer r.mu.Unlock()
	reservation, ok := r.reservations[orderID]
	if !ok || reservation.Status == domain.ReservationReleased {
		return nil
	}
	if reservation.Status == domain.ReservationConfirmed {
		return nil
	}
	stock := r.stocks[reservation.ProductID]
	stock.Available += reservation.Quantity
	stock.Reserved -= reservation.Quantity
	if stock.Reserved < 0 {
		stock.Reserved = 0
	}
	r.stocks[reservation.ProductID] = stock
	reservation.Status = domain.ReservationReleased
	r.reservations[orderID] = reservation
	return nil
}

func (r *MemoryStockRepository) ReconcileStock(ctx context.Context, productID int64) (domain.Stock, domain.Stock, bool, error) {
	stock, err := r.GetStock(ctx, productID)
	if err != nil {
		return domain.Stock{}, domain.Stock{}, false, err
	}
	return stock, stock, false, nil
}
