package apperror

// Code is a stable business error code shared by HTTP and RPC boundaries.
type Code string

const (
	CodeOK              Code = "OK"
	CodeInvalidArgument Code = "INVALID_ARGUMENT"
	CodeUnauthorized    Code = "UNAUTHORIZED"
	CodeForbidden       Code = "FORBIDDEN"
	CodeNotFound        Code = "NOT_FOUND"
	CodeConflict        Code = "CONFLICT"
	CodeTooManyRequests Code = "TOO_MANY_REQUESTS"
	CodeInternal        Code = "INTERNAL"

	CodeProductNotFound Code = "PRODUCT_NOT_FOUND"
	CodeProductOffShelf Code = "PRODUCT_OFF_SHELF"

	CodeStockNotFound        Code = "STOCK_NOT_FOUND"
	CodeStockInsufficient    Code = "STOCK_INSUFFICIENT"
	CodeStockReserveFailed   Code = "STOCK_RESERVE_FAILED"
	CodeStockReconcileFailed Code = "STOCK_RECONCILE_FAILED"

	CodeOrderNotFound      Code = "ORDER_NOT_FOUND"
	CodeOrderStatusInvalid Code = "ORDER_STATUS_INVALID"
	CodeOrderCreateFailed  Code = "ORDER_CREATE_FAILED"
	CodeOrderAlreadyPaid   Code = "ORDER_ALREADY_PAID"

	CodePaymentStatusInvalid Code = "PAYMENT_STATUS_INVALID"
	CodePaymentFailed        Code = "PAYMENT_FAILED"

	CodeRefundNotAllowed    Code = "REFUND_NOT_ALLOWED"
	CodeRefundNotFound      Code = "REFUND_NOT_FOUND"
	CodeRefundStatusInvalid Code = "REFUND_STATUS_INVALID"

	CodeMerchantNotFound      Code = "MERCHANT_NOT_FOUND"
	CodeMerchantNotBound      Code = "MERCHANT_NOT_BOUND"
	CodeMerchantApplyPending  Code = "MERCHANT_APPLY_PENDING"
	CodeMerchantApplyRejected Code = "MERCHANT_APPLY_REJECTED"
)

var defaultMessages = map[Code]string{
	CodeOK:              "success",
	CodeInvalidArgument: "invalid argument",
	CodeUnauthorized:    "unauthorized",
	CodeForbidden:       "forbidden",
	CodeNotFound:        "not found",
	CodeConflict:        "conflict",
	CodeTooManyRequests: "too many requests",
	CodeInternal:        "internal error",

	CodeProductNotFound: "product not found",
	CodeProductOffShelf: "product is off shelf",

	CodeStockNotFound:        "stock not found",
	CodeStockInsufficient:    "stock insufficient",
	CodeStockReserveFailed:   "stock reserve failed",
	CodeStockReconcileFailed: "stock reconcile failed",

	CodeOrderNotFound:      "order not found",
	CodeOrderStatusInvalid: "order status invalid",
	CodeOrderCreateFailed:  "order create failed",
	CodeOrderAlreadyPaid:   "order already paid",

	CodePaymentStatusInvalid: "payment status invalid",
	CodePaymentFailed:        "payment failed",

	CodeRefundNotAllowed:    "refund not allowed",
	CodeRefundNotFound:      "refund not found",
	CodeRefundStatusInvalid: "refund status invalid",

	CodeMerchantNotFound:      "merchant not found",
	CodeMerchantNotBound:      "merchant not bound",
	CodeMerchantApplyPending:  "merchant application pending",
	CodeMerchantApplyRejected: "merchant application rejected",
}

// DefaultMessage returns the stable fallback message for a business code.
func DefaultMessage(code Code) string {
	if msg, ok := defaultMessages[code]; ok {
		return msg
	}
	return defaultMessages[CodeInternal]
}
