package handler

import (
	"flash-mall/app/gateway/hertz/internal/application/orderquery"
	"flash-mall/app/gateway/hertz/internal/application/useraddress"
)

type CreateOrderReq struct {
	RequestID        string `json:"request_id"`
	UserID           int64  `json:"user_id,omitempty"`
	ProductID        int64  `json:"product_id"`
	Amount           int64  `json:"amount"`
	ExpectedPriceFen int64  `json:"expected_price_fen,omitempty"`
}

type CreateOrderResp = orderquery.CreateOrderResult

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

type UserAddressItem = useraddress.Address
type UserAddressListResp = useraddress.ListResult
type UserAddressUpsertReq = useraddress.UpsertInput
type UserAddressUpsertResp = useraddress.UpsertResult

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

type OrderListItem = orderquery.ListItem

type OrderListResp struct {
	Items []OrderListItem `json:"items"`
}

type OrderDetailResp = orderquery.Detail

type MerchantOrderListReq = orderquery.MerchantListQuery

type MerchantOrderItem = orderquery.BackofficeOrderItem

type MerchantOrderListResp = orderquery.BackofficeOrderList

type MerchantShipOrderReq struct {
	OrderID string `json:"order_id"`
}

type MerchantShipOrderResp struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

type MerchantRefundListReq = orderquery.RefundListQuery

type MerchantRefundItem = orderquery.RefundItem

type MerchantRefundListResp = orderquery.RefundList
