package domain

import "flash-mall/app/common/apperror"

var (
	ErrProductIDRequired = apperror.New(apperror.CodeInvalidArgument, "product_id is required")
	ErrOrderIDRequired   = apperror.New(apperror.CodeInvalidArgument, "order_id is required")
	ErrQuantityInvalid   = apperror.New(apperror.CodeInvalidArgument, "quantity must be positive")
	ErrStockNotFound     = apperror.New(apperror.CodeStockNotFound, "stock not found")
	ErrStockInsufficient = apperror.New(apperror.CodeStockInsufficient, "stock insufficient")
)
