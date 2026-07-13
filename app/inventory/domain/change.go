package domain

type StockChangeMeta struct {
	OrderID    string
	Reason     string
	RequestID  string
	TraceID    string
	UserID     int64
	MerchantID int64
	Role       string
}
