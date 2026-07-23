package handler

import (
	"flash-mall/app/gateway/hertz/internal/application/adminops"
	"flash-mall/app/gateway/hertz/internal/application/orderquery"
	"flash-mall/app/gateway/hertz/internal/application/promotion"
	"flash-mall/app/gateway/hertz/internal/application/reconciliation"
	"flash-mall/app/gateway/hertz/internal/application/supplier"
)

type AdminOrderListReq = orderquery.AdminListQuery

type AdminOrderItem = orderquery.BackofficeOrderItem

type AdminOrderListResp = orderquery.BackofficeOrderList

type AdminOrderStatusLogItem = orderquery.StatusLogItem

type AdminOrderStatusLogResp = orderquery.StatusLogList

type AdminRefundListReq = orderquery.RefundListQuery

type AdminRefundItem = orderquery.RefundItem

type AdminRefundListResp = orderquery.RefundList

type AdminRefundAuditReq struct {
	RefundID string `json:"refund_id"`
	Approve  bool   `json:"approve"`
	Remark   string `json:"remark,omitempty"`
}

type AdminRefundAuditResp struct {
	RefundID string `json:"refund_id"`
	Status   string `json:"status"`
}

type AdminDashboardStats = adminops.DashboardStats

type AdminReconciliationReq = reconciliation.Query
type AdminReconciliationItem = reconciliation.Issue
type AdminReconciliationResp = reconciliation.List

type AdminEventListReq = adminops.EventQuery
type AdminEventItem = adminops.Event
type AdminEventListResp = adminops.EventList

type AdminEventRetryReq struct {
	EventID string `json:"event_id"`
}

type AdminEventRetryResp struct {
	EventID string `json:"event_id"`
	Status  string `json:"status"`
}

type AdminSupplierItem = supplier.Record

type AdminSupplierListResp struct {
	Items    []AdminSupplierItem `json:"items"`
	Total    int64               `json:"total"`
	Page     int64               `json:"page"`
	PageSize int64               `json:"page_size"`
}

type AdminSupplierCreateReq = supplier.CreateInput

type AdminSupplierCreateResp struct {
	SupplierID int64 `json:"supplier_id"`
}

type AdminSupplierUpdateReq = supplier.UpdateInput

type AdminPromotionItem = promotion.Item

type AdminPromotionListResp struct {
	Items    []AdminPromotionItem `json:"items"`
	Total    int64                `json:"total"`
	Page     int64                `json:"page"`
	PageSize int64                `json:"page_size"`
}

type AdminPromotionCreateReq = promotion.CreateInput

type AdminPromotionCreateResp struct {
	PromotionID int64 `json:"promotion_id"`
}

type AdminPromotionUpdateReq = promotion.UpdateInput
