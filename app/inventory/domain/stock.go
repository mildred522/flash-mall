package domain

type Stock struct {
	ProductID int64
	Available int64
	Reserved  int64
	Total     int64
}

func NewStock(productID int64, total int64) Stock {
	if total < 0 {
		total = 0
	}
	return Stock{ProductID: productID, Available: total, Total: total}
}

func (s Stock) CanReserve(quantity int64) bool {
	return quantity > 0 && s.Available >= quantity
}
