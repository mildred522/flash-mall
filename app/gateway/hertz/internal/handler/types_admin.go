package handler

type AdminOrderListReq struct {
	Page        int64  `json:"page,omitempty"`
	PageSize    int64  `json:"page_size,omitempty"`
	MerchantID  int64  `json:"merchant_id,omitempty"`
	ProductID   int64  `json:"product_id,omitempty"`
	Status      int64  `json:"status,omitempty"`
	UserID      int64  `json:"user_id,omitempty"`
	ProductName string `json:"product_name,omitempty"`
	CreatedFrom string `json:"created_from,omitempty"`
	CreatedTo   string `json:"created_to,omitempty"`
	OrderID     string `json:"order_id,omitempty"`
}

type AdminOrderItem = MerchantOrderItem

type AdminOrderListResp struct {
	Items []AdminOrderItem `json:"items"`
	Total int64            `json:"total"`
}

type AdminOrderStatusLogItem struct {
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

type AdminOrderStatusLogResp struct {
	Items []AdminOrderStatusLogItem `json:"items"`
}

type AdminRefundListReq struct {
	Page       int64  `json:"page,omitempty"`
	PageSize   int64  `json:"page_size,omitempty"`
	MerchantID int64  `json:"merchant_id,omitempty"`
	Status     int64  `json:"status,omitempty"`
	UserID     int64  `json:"user_id,omitempty"`
	OrderID    string `json:"order_id,omitempty"`
}

type AdminRefundItem = MerchantRefundItem

type AdminRefundListResp struct {
	Items []AdminRefundItem `json:"items"`
	Total int64             `json:"total"`
}

type AdminRefundAuditReq struct {
	RefundID string `json:"refund_id"`
	Approve  bool   `json:"approve"`
	Remark   string `json:"remark,omitempty"`
}

type AdminRefundAuditResp struct {
	RefundID string `json:"refund_id"`
	Status   string `json:"status"`
}

type AdminDashboardStats struct {
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

type AdminReconciliationReq struct {
	Page      int64  `json:"page,omitempty"`
	PageSize  int64  `json:"page_size,omitempty"`
	Status    int64  `json:"status,omitempty"`
	IssueType string `json:"issue_type,omitempty"`
	OrderID   string `json:"order_id,omitempty"`
}

type AdminReconciliationItem struct {
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

type AdminReconciliationResp struct {
	Items []AdminReconciliationItem `json:"items"`
	Total int64                     `json:"total"`
}

type AdminEventListReq struct {
	Page        int64  `json:"page,omitempty"`
	PageSize    int64  `json:"page_size,omitempty"`
	Status      int64  `json:"status,omitempty"`
	EventType   string `json:"event_type,omitempty"`
	AggregateID string `json:"aggregate_id,omitempty"`
}

type AdminEventItem struct {
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

type AdminEventListResp struct {
	Items []AdminEventItem `json:"items"`
	Total int64            `json:"total"`
}

type AdminEventRetryReq struct {
	EventID string `json:"event_id"`
}

type AdminEventRetryResp struct {
	EventID string `json:"event_id"`
	Status  string `json:"status"`
}

type AdminSupplierItem struct {
	SupplierID     int64  `json:"supplier_id"`
	Name           string `json:"name"`
	Status         int64  `json:"status"`
	StatusText     string `json:"status_text"`
	ProductCount   int64  `json:"product_count"`
	ActiveProducts int64  `json:"active_products"`
}

type AdminSupplierListResp struct {
	Items    []AdminSupplierItem `json:"items"`
	Total    int64               `json:"total"`
	Page     int64               `json:"page"`
	PageSize int64               `json:"page_size"`
}

type AdminSupplierCreateReq struct {
	Name   string `json:"name"`
	Status int64  `json:"status,omitempty"`
}

type AdminSupplierCreateResp struct {
	SupplierID int64 `json:"supplier_id"`
}

type AdminSupplierUpdateReq struct {
	SupplierID int64  `json:"supplier_id"`
	Name       string `json:"name,omitempty"`
	Status     *int64 `json:"status,omitempty"`
}

type AdminPromotionItem struct {
	PromotionID      int64  `json:"promotion_id"`
	ProductID        int64  `json:"product_id"`
	ProductName      string `json:"product_name"`
	OriginPriceFen   int64  `json:"origin_price_fen"`
	SalePriceFen     int64  `json:"sale_price_fen"`
	Type             string `json:"type"`
	DiscountValue    int64  `json:"discount_value"`
	ThresholdAmount  int64  `json:"threshold_amount"`
	StartsAt         string `json:"starts_at"`
	EndsAt           string `json:"ends_at"`
	EffectStatus     string `json:"effect_status"`
	EffectStatusText string `json:"effect_status_text"`
	Status           int64  `json:"status"`
	StatusText       string `json:"status_text"`
}

type AdminPromotionListResp struct {
	Items    []AdminPromotionItem `json:"items"`
	Total    int64                `json:"total"`
	Page     int64                `json:"page"`
	PageSize int64                `json:"page_size"`
}

type AdminPromotionCreateReq struct {
	ProductID       int64  `json:"product_id"`
	Type            string `json:"type,omitempty"`
	DiscountValue   int64  `json:"discount_value"`
	ThresholdAmount int64  `json:"threshold_amount,omitempty"`
	StartsAt        string `json:"starts_at,omitempty"`
	EndsAt          string `json:"ends_at,omitempty"`
	Status          int64  `json:"status,omitempty"`
}

type AdminPromotionCreateResp struct {
	PromotionID int64 `json:"promotion_id"`
}

type AdminPromotionUpdateReq struct {
	PromotionID     int64   `json:"promotion_id"`
	ProductID       *int64  `json:"product_id,omitempty"`
	DiscountValue   *int64  `json:"discount_value,omitempty"`
	ThresholdAmount *int64  `json:"threshold_amount,omitempty"`
	StartsAt        *string `json:"starts_at,omitempty"`
	EndsAt          *string `json:"ends_at,omitempty"`
	Status          *int64  `json:"status,omitempty"`
}
