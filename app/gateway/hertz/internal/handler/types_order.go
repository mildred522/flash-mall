package handler

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
