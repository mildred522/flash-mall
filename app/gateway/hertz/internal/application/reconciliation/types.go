package reconciliation

import "context"

type Query struct {
	Page      int64  `json:"page,omitempty"`
	PageSize  int64  `json:"page_size,omitempty"`
	Status    int64  `json:"status,omitempty"`
	IssueType string `json:"issue_type,omitempty"`
	OrderID   string `json:"order_id,omitempty"`
}

type Issue struct {
	ID                int64  `json:"id"`
	IssueType         string `json:"issue_type"`
	OrderID           string `json:"order_id"`
	PaymentOrderID    string `json:"payment_order_id"`
	RefundOrderID     string `json:"refund_order_id"`
	ExpectedAmountFen int64  `json:"expected_amount_fen"`
	ActualAmountFen   int64  `json:"actual_amount_fen"`
	Severity          int64  `json:"severity"`
	Status            int64  `json:"status"`
	Detail            string `json:"detail"`
	CreateTime        string `json:"create_time"`
}

type List struct {
	Items []Issue `json:"items"`
	Total int64   `json:"total"`
}

type Repository interface {
	List(context.Context, Query) (List, error)
	Scan(context.Context) (int64, error)
}
