package handler

type InventorySummary struct {
	ProductID int64  `json:"product_id"`
	Available int64  `json:"available"`
	Reserved  int64  `json:"reserved"`
	Total     int64  `json:"total"`
	Source    string `json:"source"`
}
