package orderquery

import (
	"context"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/orderstatus"
	"flash-mall/app/common/paymentstatus"
)

type ListItem struct {
	OrderID          string `json:"order_id"`
	ProductID        int64  `json:"product_id"`
	ProductName      string `json:"product_name"`
	ImageURL         string `json:"image_url"`
	Amount           int64  `json:"amount"`
	Status           int64  `json:"status"`
	StatusText       string `json:"status_text"`
	PayableAmountFen int64  `json:"payable_amount_fen"`
	CreateTime       string `json:"create_time"`
}

type Detail struct {
	OrderID            string `json:"order_id"`
	UserID             int64  `json:"user_id"`
	MerchantID         int64  `json:"merchant_id"`
	MerchantName       string `json:"merchant_name"`
	ProductID          int64  `json:"product_id"`
	ProductName        string `json:"product_name"`
	ImageURL           string `json:"image_url"`
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

type PaymentOrder struct {
	OrderID          string
	UserID           int64
	OrderStatus      int64
	PaymentOrderID   string
	PaymentStatus    int64
	OutTradeNo       string
	PayableAmountFen int64
	ExpiresAt        int64
}

type CreateOrderResult struct {
	OrderID          string `json:"order_id"`
	Status           string `json:"status"`
	PayableAmountFen int64  `json:"payable_amount_fen"`
	PaymentOrderID   string `json:"payment_order_id"`
}

type RequestStatus struct {
	RequestID string
	OrderID   string
	Status    string
}

type Repository interface {
	ListByUser(context.Context, int64, int64) ([]ListItem, error)
	DetailByUser(context.Context, string, int64) (Detail, bool, error)
	PaymentByUser(context.Context, string, int64) (PaymentOrder, bool, error)
	PaymentByID(context.Context, string, int64) (PaymentOrder, bool, error)
	PaymentByClaims(context.Context, string, string, string) (PaymentOrder, bool, error)
	OrderStatusByRequest(context.Context, string, int64) (string, int64, bool, error)
	OrderIDByRequest(context.Context, string, int64) (string, bool, error)
	CreateResultByOrder(context.Context, string, int64) (CreateOrderResult, int64, bool, error)
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) List(ctx context.Context, userID int64) ([]ListItem, error) {
	items, err := s.repository.ListByUser(ctx, userID, 50)
	for index := range items {
		items[index].StatusText = orderstatus.Text(items[index].Status)
	}
	return items, err
}

func (s *Service) Detail(ctx context.Context, orderID string, userID int64) (Detail, error) {
	detail, found, err := s.repository.DetailByUser(ctx, orderID, userID)
	if err != nil {
		return Detail{}, err
	}
	if !found {
		return Detail{}, apperror.New(apperror.CodeOrderNotFound, "order not found")
	}
	detail.StatusText = orderstatus.Text(detail.Status)
	detail.PaymentStatusText = paymentstatus.Text(detail.PaymentStatus)
	return detail, nil
}

func (s *Service) Payment(ctx context.Context, orderID string, userID int64) (PaymentOrder, error) {
	payment, found, err := s.repository.PaymentByUser(ctx, orderID, userID)
	if err != nil {
		return PaymentOrder{}, err
	}
	if !found {
		return PaymentOrder{}, apperror.New(apperror.CodeOrderNotFound, "order not found")
	}
	return payment, nil
}

func (s *Service) PaymentByID(ctx context.Context, paymentOrderID string, userID int64) (PaymentOrder, error) {
	payment, found, err := s.repository.PaymentByID(ctx, paymentOrderID, userID)
	return requirePayment(payment, found, err)
}

func (s *Service) PaymentByClaims(ctx context.Context, paymentOrderID, orderID, outTradeNo string) (PaymentOrder, error) {
	payment, found, err := s.repository.PaymentByClaims(ctx, paymentOrderID, orderID, outTradeNo)
	return requirePayment(payment, found, err)
}

func (s *Service) StatusByRequest(ctx context.Context, requestID string, userID int64) (RequestStatus, bool, error) {
	orderID, status, found, err := s.repository.OrderStatusByRequest(ctx, requestID, userID)
	if err != nil || !found {
		return RequestStatus{}, found, err
	}
	return RequestStatus{RequestID: requestID, OrderID: orderID, Status: orderstatus.Text(status)}, true, nil
}

func requirePayment(payment PaymentOrder, found bool, err error) (PaymentOrder, error) {
	if err != nil {
		return PaymentOrder{}, err
	}
	if !found {
		return PaymentOrder{}, apperror.New(apperror.CodeNotFound, "payment order not found")
	}
	return payment, nil
}

func (s *Service) CreateByRequest(ctx context.Context, requestID string, userID int64) (CreateOrderResult, bool, error) {
	orderID, found, err := s.repository.OrderIDByRequest(ctx, requestID, userID)
	if err != nil || !found {
		return CreateOrderResult{}, false, err
	}
	result, err := s.CreateByOrder(ctx, orderID, userID)
	return result, true, err
}

func (s *Service) CreateByOrder(ctx context.Context, orderID string, userID int64) (CreateOrderResult, error) {
	result, status, found, err := s.repository.CreateResultByOrder(ctx, orderID, userID)
	if err != nil {
		return CreateOrderResult{}, err
	}
	if !found {
		return CreateOrderResult{}, apperror.New(apperror.CodeOrderNotFound, "order not found")
	}
	result.Status = orderstatus.Text(status)
	if result.PaymentOrderID == "" {
		result.PaymentOrderID = "pay:" + result.OrderID
	}
	return result, nil
}
