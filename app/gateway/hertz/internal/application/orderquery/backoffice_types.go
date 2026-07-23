package orderquery

import "context"

type AdminListQuery struct {
	Page, PageSize, MerchantID, ProductID, Status, UserID int64
	ProductName, CreatedFrom, CreatedTo, OrderID          string
}

type MerchantListQuery struct {
	MerchantID, Page, PageSize, Status, UserID, ProductID int64
	OrderID                                               string
}

type RefundListQuery struct {
	MerchantID, Page, PageSize, Status, UserID int64
	OrderID                                    string
}

type BackofficeOrderItem struct {
	OrderID          string `json:"order_id"`
	UserID           int64  `json:"user_id"`
	MerchantID       int64  `json:"merchant_id"`
	MerchantName     string `json:"merchant_name"`
	ProductID        int64  `json:"product_id"`
	ProductName      string `json:"product_name"`
	Amount           int64  `json:"amount"`
	Status           int64  `json:"status"`
	StatusText       string `json:"status_text"`
	PayableAmountFen int64  `json:"payable_amount_fen"`
	CreateTime       string `json:"create_time"`
}

type BackofficeOrderList struct {
	Items []BackofficeOrderItem `json:"items"`
	Total int64                 `json:"total"`
}

type StatusLogItem struct {
	ID             int64  `json:"id"`
	OrderID        string `json:"order_id"`
	FromStatus     int64  `json:"from_status"`
	FromStatusText string `json:"from_status_text"`
	ToStatus       int64  `json:"to_status"`
	ToStatusText   string `json:"to_status_text"`
	OperatorID     int64  `json:"operator_id"`
	Remark         string `json:"remark"`
	CreateTime     string `json:"create_time"`
}

type StatusLogList struct {
	Items []StatusLogItem `json:"items"`
}

type RefundItem struct {
	RefundID        string `json:"refund_id"`
	OrderID         string `json:"order_id"`
	PaymentOrderID  string `json:"payment_order_id"`
	UserID          int64  `json:"user_id"`
	MerchantID      int64  `json:"merchant_id"`
	MerchantName    string `json:"merchant_name"`
	ProductID       int64  `json:"product_id"`
	RefundAmountFen int64  `json:"refund_amount_fen"`
	Status          int64  `json:"status"`
	StatusText      string `json:"status_text"`
	Reason          string `json:"reason"`
	AuditRemark     string `json:"audit_remark"`
	OperatorID      int64  `json:"operator_id"`
	RequestTime     string `json:"request_time"`
	AuditTime       string `json:"audit_time"`
	FinishTime      string `json:"finish_time"`
}

type RefundList struct {
	Items []RefundItem `json:"items"`
	Total int64        `json:"total"`
}

type BackofficeRepository interface {
	ListOrders(context.Context, AdminListQuery) ([]BackofficeOrderItem, int64, error)
	DetailByID(context.Context, string) (Detail, bool, error)
	StatusLogs(context.Context, string) ([]StatusLogItem, error)
	ListMerchantOrders(context.Context, MerchantListQuery) ([]BackofficeOrderItem, int64, error)
	ListAdminRefunds(context.Context, RefundListQuery) ([]RefundItem, int64, error)
	ListMerchantRefunds(context.Context, RefundListQuery) ([]RefundItem, int64, error)
}
