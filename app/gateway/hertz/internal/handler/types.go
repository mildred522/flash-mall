package handler

type ProductCard struct {
	ProductID      int64  `json:"product_id"`
	Name           string `json:"name"`
	ImageURL       string `json:"image_url"`
	OriginPriceFen int64  `json:"origin_price_fen"`
	FinalPriceFen  int64  `json:"final_price_fen"`
	SupplierID     int64  `json:"supplier_id"`
	SupplierName   string `json:"supplier_name"`
	PromotionTag   string `json:"promotion_tag"`
	StockAvailable int64  `json:"stock_available"`
	StockReserved  int64  `json:"stock_reserved,omitempty"`
	StockTotal     int64  `json:"stock_total,omitempty"`
	StockSource    string `json:"stock_source,omitempty"`
	MerchantID     int64  `json:"merchant_id"`
	MerchantName   string `json:"merchant_name"`
	MerchantLogo   string `json:"merchant_logo,omitempty"`
	StoreURL       string `json:"store_url"`
	StoreStatus    int64  `json:"store_status"`
	SlotNo         int64  `json:"slot_no,omitempty"`
}

type ProductListResp struct {
	Items    []ProductCard `json:"items"`
	Total    int64         `json:"total"`
	Page     int64         `json:"page"`
	PageSize int64         `json:"page_size"`
}

type PublicStoreDetail struct {
	MerchantID   int64  `json:"merchant_id"`
	MerchantName string `json:"merchant_name"`
	LogoURL      string `json:"logo_url"`
	BannerURL    string `json:"banner_url"`
	Description  string `json:"description"`
	Status       int64  `json:"status"`
	ProductCount int64  `json:"product_count"`
}

type StoreProductListResp struct {
	Items    []ProductCard `json:"items"`
	Total    int64         `json:"total"`
	Page     int64         `json:"page"`
	PageSize int64         `json:"page_size"`
}

type ProductDetailResp struct {
	Item          ProductCard   `json:"item"`
	StoreProducts []ProductCard `json:"store_products"`
}

type ShowcaseSlot struct {
	SlotNo        int64        `json:"slot_no"`
	ProductID     int64        `json:"product_id"`
	Empty         bool         `json:"empty"`
	Valid         bool         `json:"valid"`
	InvalidReason string       `json:"invalid_reason,omitempty"`
	Product       *ProductCard `json:"product,omitempty"`
}

type ShowcaseResp struct {
	Version     int64          `json:"version"`
	OperatorID  int64          `json:"operator_id"`
	PublishTime string         `json:"publish_time"`
	Items       []ShowcaseSlot `json:"items"`
}

type AdminProductItem struct {
	ProductID         int64  `json:"product_id"`
	MerchantID        int64  `json:"merchant_id"`
	MerchantName      string `json:"merchant_name"`
	Name              string `json:"name"`
	ImageURL          string `json:"image_url"`
	OriginPriceFen    int64  `json:"origin_price_fen"`
	SalePriceFen      int64  `json:"sale_price_fen"`
	SupplierID        int64  `json:"supplier_id"`
	SupplierName      string `json:"supplier_name"`
	StockAvailable    int64  `json:"stock_available"`
	PromotionPriceFen int64  `json:"promotion_price_fen"`
	Status            int64  `json:"status"`
	StatusText        string `json:"status_text"`
	PromotionText     string `json:"promotion_text"`
}

type AdminProductListResp struct {
	Items    []AdminProductItem `json:"items"`
	Total    int64              `json:"total"`
	Page     int64              `json:"page"`
	PageSize int64              `json:"page_size"`
}

type AdminProductCardSnapshotRefreshReq struct {
	ProductID     int64 `json:"product_id,omitempty"`
	Limit         int64 `json:"limit,omitempty"`
	WindowMinutes int64 `json:"window_minutes,omitempty"`
}

type AdminProductCardSnapshotRefreshResp struct {
	ProductCount  int64 `json:"product_count"`
	Affected      int64 `json:"affected"`
	Limit         int64 `json:"limit"`
	WindowMinutes int64 `json:"window_minutes"`
}

type AdminStockSnapshotRebuildReq struct {
	ProductID int64 `json:"product_id,omitempty"`
	Limit     int64 `json:"limit,omitempty"`
}

type AdminProductUpdateReq struct {
	ProductID      int64  `json:"product_id"`
	Name           string `json:"name,omitempty"`
	ImageURL       string `json:"image_url,omitempty"`
	SalePriceFen   *int64 `json:"sale_price_fen,omitempty"`
	OriginPriceFen *int64 `json:"origin_price_fen,omitempty"`
	SupplierID     *int64 `json:"supplier_id,omitempty"`
	Status         *int64 `json:"status,omitempty"`
}

type AdminProductCreateReq struct {
	Name           string `json:"name"`
	ImageURL       string `json:"image_url,omitempty"`
	MerchantID     int64  `json:"merchant_id,omitempty"`
	OriginPriceFen int64  `json:"origin_price_fen"`
	SalePriceFen   int64  `json:"sale_price_fen"`
	StockAvailable int64  `json:"stock_available,omitempty"`
	SupplierID     int64  `json:"supplier_id,omitempty"`
	Status         int64  `json:"status,omitempty"`
}

type AdminProductCreateResp struct {
	ProductID int64 `json:"product_id"`
}

type AdminProductStockAdjustReq struct {
	ProductID int64 `json:"product_id"`
	Delta     int64 `json:"delta"`
	BucketIdx int64 `json:"bucket_idx,omitempty"`
}

type AdminProductStockAdjustResp struct {
	ProductID      int64 `json:"product_id"`
	StockAvailable int64 `json:"stock_available"`
}

type CreateOrderReq struct {
	RequestID        string `json:"request_id"`
	UserID           int64  `json:"user_id,omitempty"`
	ProductID        int64  `json:"product_id"`
	Amount           int64  `json:"amount"`
	ExpectedPriceFen int64  `json:"expected_price_fen,omitempty"`
}

type CreateOrderResp struct {
	OrderID          string `json:"order_id"`
	Status           string `json:"status"`
	PayableAmountFen int64  `json:"payable_amount_fen"`
	PaymentOrderID   string `json:"payment_order_id"`
}

type PayOrderReq struct {
	OrderID string `json:"order_id"`
}

type PayOrderResp struct {
	OrderID          string `json:"order_id"`
	PaymentOrderID   string `json:"payment_order_id,omitempty"`
	OutTradeNo       string `json:"out_trade_no,omitempty"`
	PayableAmountFen int64  `json:"payable_amount_fen,omitempty"`
	Status           string `json:"status"`
	QRURL            string `json:"qr_url,omitempty"`
	ExpiresAt        int64  `json:"expires_at,omitempty"`
}

type SandboxPaymentConfirmReq struct {
	Token string `json:"token"`
}

type PaymentStatusResp struct {
	OrderID          string `json:"order_id"`
	PaymentOrderID   string `json:"payment_order_id"`
	OutTradeNo       string `json:"out_trade_no"`
	PayableAmountFen int64  `json:"payable_amount_fen"`
	Status           string `json:"status"`
	ExpiresAt        int64  `json:"expires_at,omitempty"`
}

type PaymentCallbackReq struct {
	OrderID        string `json:"order_id"`
	PaymentOrderID string `json:"payment_order_id"`
	OutTradeNo     string `json:"out_trade_no"`
	PaidAmountFen  int64  `json:"paid_amount_fen"`
	Provider       string `json:"provider,omitempty"`
	EventID        string `json:"event_id,omitempty"`
	Timestamp      string `json:"timestamp"`
	Nonce          string `json:"nonce"`
	Signature      string `json:"signature"`
}

type OrderStatusPollResp struct {
	RequestID string `json:"request_id"`
	OrderID   string `json:"order_id,omitempty"`
	Status    string `json:"status"`
}

type UserAddressItem struct {
	AddressID     int64  `json:"address_id"`
	ReceiverName  string `json:"receiver_name"`
	ReceiverPhone string `json:"receiver_phone"`
	Province      string `json:"province"`
	City          string `json:"city"`
	District      string `json:"district"`
	Detail        string `json:"detail"`
	IsDefault     bool   `json:"is_default"`
}

type UserAddressListResp struct {
	Items []UserAddressItem `json:"items"`
}

type UserAddressUpsertReq struct {
	AddressID     int64  `json:"address_id,omitempty"`
	ReceiverName  string `json:"receiver_name"`
	ReceiverPhone string `json:"receiver_phone"`
	Province      string `json:"province,omitempty"`
	City          string `json:"city,omitempty"`
	District      string `json:"district,omitempty"`
	Detail        string `json:"detail"`
	IsDefault     bool   `json:"is_default,omitempty"`
}

type UserAddressUpsertResp struct {
	AddressID int64 `json:"address_id"`
}

type CancelOrderReq struct {
	OrderID string `json:"order_id"`
	Reason  string `json:"reason,omitempty"`
}

type CancelOrderResp struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

type RefundOrderReq struct {
	OrderID string `json:"order_id"`
	Reason  string `json:"reason,omitempty"`
}

type RefundOrderResp struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

type ConfirmReceiptReq struct {
	OrderID string `json:"order_id"`
}

type ConfirmReceiptResp struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

type OrderListItem struct {
	OrderID          string `json:"order_id"`
	ProductID        int64  `json:"product_id"`
	ProductName      string `json:"product_name"`
	Amount           int64  `json:"amount"`
	Status           int64  `json:"status"`
	StatusText       string `json:"status_text"`
	PayableAmountFen int64  `json:"payable_amount_fen"`
	CreateTime       string `json:"create_time"`
}

type OrderListResp struct {
	Items []OrderListItem `json:"items"`
}

type OrderDetailResp struct {
	OrderID            string `json:"order_id"`
	UserID             int64  `json:"user_id"`
	MerchantID         int64  `json:"merchant_id"`
	MerchantName       string `json:"merchant_name"`
	ProductID          int64  `json:"product_id"`
	ProductName        string `json:"product_name"`
	Amount             int64  `json:"amount"`
	Status             int64  `json:"status"`
	StatusText         string `json:"status_text"`
	OriginUnitPriceFen int64  `json:"origin_unit_price_fen"`
	SaleUnitPriceFen   int64  `json:"sale_unit_price_fen"`
	PayableAmountFen   int64  `json:"payable_amount_fen"`
	DiscountAmountFen  int64  `json:"discount_amount_fen"`
	PromotionType      string `json:"promotion_type"`
	PromotionTag       string `json:"promotion_tag"`
	PaymentOrderID     string `json:"payment_order_id"`
	PaymentStatus      int64  `json:"payment_status"`
	PaymentStatusText  string `json:"payment_status_text"`
	CreateTime         string `json:"create_time"`
}

type MerchantOrderListReq struct {
	MerchantID int64  `json:"merchant_id,omitempty"`
	Page       int64  `json:"page,omitempty"`
	PageSize   int64  `json:"page_size,omitempty"`
	Status     int64  `json:"status,omitempty"`
	UserID     int64  `json:"user_id,omitempty"`
	ProductID  int64  `json:"product_id,omitempty"`
	OrderID    string `json:"order_id,omitempty"`
}

type MerchantOrderItem struct {
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

type MerchantOrderListResp struct {
	Items []MerchantOrderItem `json:"items"`
	Total int64               `json:"total"`
}

type MerchantShipOrderReq struct {
	OrderID string `json:"order_id"`
}

type MerchantShipOrderResp struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

type MerchantRefundListReq struct {
	MerchantID int64  `json:"merchant_id,omitempty"`
	Page       int64  `json:"page,omitempty"`
	PageSize   int64  `json:"page_size,omitempty"`
	Status     int64  `json:"status,omitempty"`
	UserID     int64  `json:"user_id,omitempty"`
	OrderID    string `json:"order_id,omitempty"`
}

type MerchantRefundItem struct {
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

type MerchantRefundListResp struct {
	Items []MerchantRefundItem `json:"items"`
	Total int64                `json:"total"`
}

type MerchantMeItem struct {
	MerchantID int64  `json:"merchant_id"`
	Name       string `json:"name"`
	Role       string `json:"role"`
	Status     int64  `json:"status"`
}

type MerchantMeResp struct {
	Items []MerchantMeItem `json:"items"`
}

type MerchantStoreProfile struct {
	MerchantID   int64  `json:"merchant_id"`
	MerchantName string `json:"merchant_name"`
	LogoURL      string `json:"logo_url"`
	BannerURL    string `json:"banner_url"`
	Description  string `json:"description"`
	Version      int64  `json:"version"`
}

type merchantStoreUpdateReq struct {
	LogoURL         string `json:"logo_url"`
	BannerURL       string `json:"banner_url"`
	Description     string `json:"description"`
	ExpectedVersion int64  `json:"expected_version"`
}

type MerchantApplyReq struct {
	MerchantName string `json:"merchant_name"`
	ContactPhone string `json:"contact_phone,omitempty"`
}

type MerchantApplyResp struct {
	ApplyID int64  `json:"apply_id"`
	Status  string `json:"status"`
}

type MerchantApplicationItem struct {
	ApplyID      int64  `json:"apply_id"`
	MerchantName string `json:"merchant_name"`
	ContactPhone string `json:"contact_phone"`
	Status       int64  `json:"status"`
	StatusText   string `json:"status_text"`
	MerchantID   int64  `json:"merchant_id"`
	AuditRemark  string `json:"audit_remark"`
	CreateTime   string `json:"create_time"`
	AuditTime    string `json:"audit_time"`
}

type MerchantApplicationResp struct {
	Application *MerchantApplicationItem `json:"application"`
}

type AdminMerchantApplicationListReq struct {
	Status   int64 `json:"status"`
	Page     int64 `json:"page"`
	PageSize int64 `json:"page_size"`
}

type AdminMerchantApplicationItem struct {
	MerchantApplicationItem
	UserID     int64 `json:"user_id"`
	OperatorID int64 `json:"operator_id"`
}

type AdminMerchantApplicationListResp struct {
	Items    []AdminMerchantApplicationItem `json:"items"`
	Total    int64                          `json:"total"`
	Page     int64                          `json:"page"`
	PageSize int64                          `json:"page_size"`
}

type MerchantDashboardStatsResp struct {
	MerchantID       int64 `json:"merchant_id"`
	OrderCount       int64 `json:"order_count"`
	PaidOrderCount   int64 `json:"paid_order_count"`
	ShipPendingCount int64 `json:"ship_pending_count"`
	RefundPending    int64 `json:"refund_pending_count"`
	SalesAmountFen   int64 `json:"sales_amount_fen"`
}

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

type InventorySummary struct {
	ProductID int64  `json:"product_id"`
	Available int64  `json:"available"`
	Reserved  int64  `json:"reserved"`
	Total     int64  `json:"total"`
	Source    string `json:"source"`
}

type InventoryReserveReq struct {
	OrderID   string `json:"order_id"`
	ProductID int64  `json:"product_id"`
	Quantity  int64  `json:"quantity"`
}

type InventoryReleaseReq struct {
	OrderID string `json:"order_id"`
	Reason  string `json:"reason,omitempty"`
}

type InventoryConfirmDeductReq struct {
	OrderID string `json:"order_id"`
}

type InventoryOperationResp struct {
	OrderID   string `json:"order_id"`
	ProductID int64  `json:"product_id,omitempty"`
	Quantity  int64  `json:"quantity,omitempty"`
	Status    string `json:"status"`
	Source    string `json:"source"`
}
