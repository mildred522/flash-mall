package adminops

import "context"

const ReasonInvalidArgument = "invalid_argument"

type DashboardStats struct {
	TotalOrders        int64 `json:"total_orders"`
	TotalRevenueFen    int64 `json:"total_revenue_fen"`
	TotalUsers         int64 `json:"total_users"`
	TotalProducts      int64 `json:"total_products"`
	TotalSuppliers     int64 `json:"total_suppliers"`
	TotalPromotions    int64 `json:"total_promotions"`
	ActivePromotions   int64 `json:"active_promotions"`
	LowStockProducts   int64 `json:"low_stock_products"`
	OutOfStockProducts int64 `json:"out_of_stock_products"`
	PendingOrders      int64 `json:"pending_orders"`
	PaidOrders         int64 `json:"paid_orders"`
	ShippedOrders      int64 `json:"shipped_orders"`
	CompletedOrders    int64 `json:"completed_orders"`
	RefundRequested    int64 `json:"refund_requested"`
	RefundedOrders     int64 `json:"refunded_orders"`
	OpenReconIssues    int64 `json:"open_reconciliation_issues"`
	PendingEvents      int64 `json:"pending_events"`
	DeadEvents         int64 `json:"dead_events"`
}

type EventQuery struct {
	Page        int64  `json:"page,omitempty"`
	PageSize    int64  `json:"page_size,omitempty"`
	Status      int64  `json:"status,omitempty"`
	EventType   string `json:"event_type,omitempty"`
	AggregateID string `json:"aggregate_id,omitempty"`
}

type Event struct {
	ID           int64  `json:"id"`
	EventID      string `json:"event_id"`
	EventType    string `json:"event_type"`
	AggregateID  string `json:"aggregate_id"`
	Status       int64  `json:"status"`
	AttemptCount int64  `json:"attempt_count"`
	LastError    string `json:"last_error"`
	CreateTime   string `json:"create_time"`
	UpdateTime   string `json:"update_time"`
}

type EventList struct {
	Items []Event `json:"items"`
	Total int64   `json:"total"`
}

type Repository interface {
	Dashboard(context.Context) (DashboardStats, error)
	Events(context.Context, EventQuery) (EventList, error)
	RetryEvent(context.Context, string) error
}
