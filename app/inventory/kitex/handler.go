package main

import (
	"context"

	"flash-mall/app/common/apperror"
	"flash-mall/app/inventory/domain"
	common "flash-mall/app/inventory/kitex/kitex_gen/flashmall/common"
	inventory "flash-mall/app/inventory/kitex/kitex_gen/flashmall/inventory"
	"flash-mall/app/inventory/service"
)

// InventoryServiceImpl adapts the generated Kitex interface to the domain service.
type InventoryServiceImpl struct {
	svc *service.Service
}

func NewInventoryServiceImpl(svc *service.Service) *InventoryServiceImpl {
	return &InventoryServiceImpl{svc: svc}
}

func (s *InventoryServiceImpl) GetStock(ctx context.Context, req *inventory.GetStockRequest) (*inventory.GetStockResponse, error) {
	stock, err := s.svc.GetStock(ctx, req.GetProductId())
	if err != nil {
		return nil, toBizException(err, req.GetMeta())
	}
	return &inventory.GetStockResponse{Stock: toStockDTO(stock)}, nil
}

func (s *InventoryServiceImpl) SeedStock(ctx context.Context, req *inventory.SeedStockRequest) (*common.Empty, error) {
	if err := s.svc.SeedStock(ctx, req.GetProductId(), req.GetTotal(), int(req.GetShardCount())); err != nil {
		return nil, toBizException(err, req.GetMeta())
	}
	return &common.Empty{}, nil
}

func (s *InventoryServiceImpl) ReserveStock(ctx context.Context, req *inventory.ReserveStockRequest) (*common.Empty, error) {
	if err := s.svc.ReserveStock(ctx, req.GetOrderId(), req.GetProductId(), req.GetQuantity()); err != nil {
		return nil, toBizException(err, req.GetMeta())
	}
	return &common.Empty{}, nil
}

func (s *InventoryServiceImpl) ConfirmDeduct(ctx context.Context, req *inventory.ConfirmDeductRequest) (*common.Empty, error) {
	if err := s.svc.ConfirmDeduct(ctx, req.GetOrderId()); err != nil {
		return nil, toBizException(err, req.GetMeta())
	}
	return &common.Empty{}, nil
}

func (s *InventoryServiceImpl) ReleaseStock(ctx context.Context, req *inventory.ReleaseStockRequest) (*common.Empty, error) {
	if err := s.svc.ReleaseStock(ctx, req.GetOrderId(), req.GetReason()); err != nil {
		return nil, toBizException(err, req.GetMeta())
	}
	return &common.Empty{}, nil
}

func (s *InventoryServiceImpl) ReconcileStock(ctx context.Context, req *inventory.ReconcileStockRequest) (*inventory.ReconcileStockResponse, error) {
	before, after, changed, err := s.svc.ReconcileStock(ctx, req.GetProductId())
	if err != nil {
		return nil, toBizException(err, req.GetMeta())
	}
	return &inventory.ReconcileStockResponse{Before: toStockDTO(before), After: toStockDTO(after), Changed: changed}, nil
}

func toStockDTO(stock domain.Stock) *inventory.StockDTO {
	return &inventory.StockDTO{
		ProductId: stock.ProductID,
		Available: stock.Available,
		Reserved:  stock.Reserved,
		Total:     stock.Total,
	}
}

func toBizException(err error, meta *common.RequestMeta) *common.BizException {
	appErr := apperror.FromError(err)
	biz := &common.BizException{Code: toIDLCode(appErr.Code), Message: appErr.Message}
	if meta != nil && meta.IsSetRequestId() {
		requestID := meta.GetRequestId()
		biz.RequestId = &requestID
	}
	return biz
}

func toIDLCode(code apperror.Code) common.ErrorCode {
	switch code {
	case apperror.CodeInvalidArgument:
		return common.ErrorCode_INVALID_ARGUMENT
	case apperror.CodeUnauthorized:
		return common.ErrorCode_UNAUTHORIZED
	case apperror.CodeForbidden:
		return common.ErrorCode_FORBIDDEN
	case apperror.CodeNotFound:
		return common.ErrorCode_NOT_FOUND
	case apperror.CodeConflict:
		return common.ErrorCode_CONFLICT
	case apperror.CodeTooManyRequests:
		return common.ErrorCode_TOO_MANY_REQUESTS
	case apperror.CodeProductNotFound:
		return common.ErrorCode_PRODUCT_NOT_FOUND
	case apperror.CodeProductOffShelf:
		return common.ErrorCode_PRODUCT_OFF_SHELF
	case apperror.CodeStockNotFound:
		return common.ErrorCode_STOCK_NOT_FOUND
	case apperror.CodeStockInsufficient:
		return common.ErrorCode_STOCK_INSUFFICIENT
	case apperror.CodeStockReserveFailed:
		return common.ErrorCode_STOCK_RESERVE_FAILED
	case apperror.CodeStockReconcileFailed:
		return common.ErrorCode_STOCK_RECONCILE_FAILED
	case apperror.CodeOrderNotFound:
		return common.ErrorCode_ORDER_NOT_FOUND
	case apperror.CodeOrderStatusInvalid:
		return common.ErrorCode_ORDER_STATUS_INVALID
	case apperror.CodeOrderCreateFailed:
		return common.ErrorCode_ORDER_CREATE_FAILED
	case apperror.CodeOrderAlreadyPaid:
		return common.ErrorCode_ORDER_ALREADY_PAID
	case apperror.CodePaymentStatusInvalid:
		return common.ErrorCode_PAYMENT_STATUS_INVALID
	case apperror.CodePaymentFailed:
		return common.ErrorCode_PAYMENT_FAILED
	case apperror.CodeRefundNotAllowed:
		return common.ErrorCode_REFUND_NOT_ALLOWED
	case apperror.CodeRefundNotFound:
		return common.ErrorCode_REFUND_NOT_FOUND
	case apperror.CodeRefundStatusInvalid:
		return common.ErrorCode_REFUND_STATUS_INVALID
	case apperror.CodeMerchantNotFound:
		return common.ErrorCode_MERCHANT_NOT_FOUND
	case apperror.CodeMerchantNotBound:
		return common.ErrorCode_MERCHANT_NOT_BOUND
	case apperror.CodeMerchantApplyPending:
		return common.ErrorCode_MERCHANT_APPLY_PENDING
	case apperror.CodeMerchantApplyRejected:
		return common.ErrorCode_MERCHANT_APPLY_REJECTED
	default:
		return common.ErrorCode_INTERNAL
	}
}
