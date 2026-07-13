package handler

const (
	adminAuditResultSuccess = "success"
	adminAuditResultFail    = "fail"
)

const (
	adminAuditOrderShipped  = "admin_order_shipped"
	adminAuditOrderRefunded = "admin_order_refunded"
	adminAuditOrderClosed   = "admin_order_closed"

	adminAuditProductCreated       = "admin_product_created"
	adminAuditProductUpdated       = "admin_product_updated"
	adminAuditProductEnabled       = "admin_product_enabled"
	adminAuditProductDisabled      = "admin_product_disabled"
	adminAuditProductStockAdjusted = "admin_product_stock_adjusted"

	adminAuditSupplierCreated  = "admin_supplier_created"
	adminAuditSupplierUpdated  = "admin_supplier_updated"
	adminAuditSupplierEnabled  = "admin_supplier_enabled"
	adminAuditSupplierDisabled = "admin_supplier_disabled"

	adminAuditPromotionCreated  = "admin_promotion_created"
	adminAuditPromotionUpdated  = "admin_promotion_updated"
	adminAuditPromotionEnabled  = "admin_promotion_enabled"
	adminAuditPromotionDisabled = "admin_promotion_disabled"
)

const (
	adminAuditReasonActiveSupplierNotFound     = "active_supplier_not_found"
	adminAuditReasonHasActiveProducts          = "has_active_products"
	adminAuditReasonInsufficientOrMissingStock = "insufficient_or_missing_bucket"
	adminAuditReasonInvalidDiscount            = "invalid_discount"
	adminAuditReasonInvalidPrice               = "invalid_price"
	adminAuditReasonInvalidStatus              = "invalid_status"
	adminAuditReasonInvalidWindow              = "invalid_window"
	adminAuditReasonNotFound                   = "not_found"
	adminAuditReasonNotPaidStatus              = "not_paid_status"
	adminAuditReasonProductNotFound            = "product_not_found"
	adminAuditReasonStatusChanged              = "status_changed"
	adminAuditReasonWindowConflict             = "window_conflict"
)
