package handler

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
)

type RouteMigrationStatusResp struct {
	Groups []RouteMigrationGroup `json:"groups"`
}

type RouteMigrationGroup struct {
	Name          string   `json:"name"`
	Status        string   `json:"status"`
	LiveRoutes    []string `json:"live_routes,omitempty"`
	PlannedRoutes []string `json:"planned_routes,omitempty"`
	Notes         string   `json:"notes,omitempty"`
}

func RouteMigrationStatusHandler() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		ok(ctx, c, routeMigrationStatus())
	}
}

func routeMigrationStatus() RouteMigrationStatusResp {
	return RouteMigrationStatusResp{
		Groups: []RouteMigrationGroup{
			{
				Name:   "system",
				Status: "partial",
				LiveRoutes: []string{
					"GET /health",
					"GET /api/system/health",
					"GET /api/system/migration/routes",
					"GET /metrics",
				},
				PlannedRoutes: []string{
					"GET /monitor",
				},
				Notes: "Health, migration visibility, and aggregate Prometheus metrics are Hertz-native; deployment should keep metrics behind an internal network policy.",
			},
			{
				Name:   "shop",
				Status: "partial",
				LiveRoutes: []string{
					"GET /",
					"GET /shop",
					"GET /product/*any",
					"GET /store/*any",
					"GET /uploads/products/*any",
					"GET /uploads/stores/*any",
					"GET /api/shop/catalog",
					"GET /api/shop/products",
					"GET /api/shop/products/detail",
					"GET /api/shop/stores/detail",
					"GET /api/shop/stores/products",
					"GET /api/user/addresses",
					"POST /api/user/addresses/upsert",
				},
				Notes: "Public pages and store/showcase orchestration are Hertz-native; product reads deliberately remain on go-zero product-rpc and product_card_snapshot to avoid an extra Kitex hop on the high-frequency read path.",
			},
			{
				Name:   "auth",
				Status: "live",
				LiveRoutes: []string{
					"GET /api/admin/showcase",
					"GET /api/admin/showcase/candidates",
					"POST /api/admin/showcase/publish",
					"POST /api/auth/login",
					"POST /api/auth/login/code",
					"POST /api/auth/register",
					"POST /api/auth/refresh",
					"POST /api/auth/logout",
					"GET /api/auth/me",
				},
				Notes: "Hertz is the external auth entry; auth-api remains the credential, session, and audit owner.",
			},
			{
				Name:   "inventory",
				Status: "live",
				LiveRoutes: []string{
					"GET /api/inventory/summary",
					"GET /api/admin/inventory/stock-changes",
					"POST /api/admin/inventory/stock-snapshots/rebuild",
					"GET /api/merchant/inventory/stock-changes",
				},
				Notes: "Inventory writes are internal order-rpc to inventory-kitex commands; Hertz exposes read and role-protected audit surfaces only.",
			},
			{
				Name:   "orders",
				Status: "partial",
				LiveRoutes: []string{
					"POST /api/order/create",
					"POST /api/order/pay",
					"POST /api/order/cancel",
					"POST /api/order/refund",
					"POST /api/order/confirm-receipt",
					"GET /api/order/detail",
					"GET /api/orders",
					"GET /api/orders/detail",
					"GET /api/order/status",
					"POST /api/payment/callback",
				},
				PlannedRoutes: []string{},
				Notes:         "User order create, payment callback, status polling, cancel, refund request, confirm receipt, list, and detail enter through Hertz; timeout-close ownership remains planned.",
			},
			{
				Name:   "admin",
				Status: "partial",
				LiveRoutes: []string{
					"GET /api/admin/products",
					"GET /api/admin/products/detail",
					"POST /api/admin/products/create",
					"POST /api/admin/products/update",
					"POST /api/admin/products/stock-adjust",
					"POST /api/admin/products/card-snapshots/refresh",
					"POST /api/admin/products/image",
					"GET /api/admin/suppliers",
					"GET /api/admin/suppliers/detail",
					"POST /api/admin/suppliers/create",
					"POST /api/admin/suppliers/update",
					"GET /api/admin/promotions",
					"GET /api/admin/promotions/detail",
					"POST /api/admin/promotions/create",
					"POST /api/admin/promotions/update",
					"GET /api/admin/orders",
					"GET /api/admin/orders/detail",
					"GET /api/admin/orders/status-logs",
					"POST /api/admin/orders/ship",
					"POST /api/admin/orders/close",
					"POST /api/admin/orders/refund",
					"GET /api/admin/refunds",
					"POST /api/admin/refunds/audit",
					"GET /api/admin/dashboard/stats",
					"GET /api/admin/reconciliation/issues",
					"POST /api/admin/reconciliation/scan",
					"GET /api/admin/events",
					"POST /api/admin/events/retry",
					"GET /api/admin/users",
					"GET /api/admin/users/detail",
					"POST /api/admin/users/status",
					"GET /api/admin/security/events/recent",
				},
				PlannedRoutes: []string{
					"GET /api/admin/campaigns",
					"POST /api/admin/campaigns/upsert",
				},
				Notes: "Admin product/supplier/promotion, order/refund, dashboard, reconciliation, outbox event, user management, and auth security log operations are live; campaign surfaces still need migration.",
			},
			{
				Name:   "merchant",
				Status: "partial",
				LiveRoutes: []string{
					"GET /api/merchant/me",
					"POST /api/merchant/apply",
					"GET /api/merchant/store/profile",
					"POST /api/merchant/store/profile",
					"POST /api/merchant/store/assets",
					"GET /api/merchant/dashboard/stats",
					"GET /api/merchant/products",
					"POST /api/merchant/products/create",
					"POST /api/merchant/products/update",
					"POST /api/merchant/products/stock-adjust",
					"GET /api/merchant/orders",
					"POST /api/merchant/orders/ship",
					"GET /api/merchant/refunds",
				},
				PlannedRoutes: []string{
					"GET /api/merchant/inventory/stock-changes",
				},
				Notes: "Merchant store/profile and operational surfaces are Hertz-native. Product reads remain go-zero RPC; inventory write commands remain internal Kitex inventory commands with idempotency and audit independent of transport.",
			},
			{
				Name:   "compatibility",
				Status: "temporary",
				LiveRoutes: []string{
					"GET /api/catalog",
					"GET /api/gateway/health",
					"GET /api/gateway/products",
					"GET /api/gateway/products/detail",
					"GET /api/gateway/inventory/summary",
					"POST /api/gateway/order/create",
					"POST /api/gateway/order/pay",
					"POST /api/gateway/order/cancel",
					"POST /api/gateway/order/refund",
					"POST /api/gateway/order/confirm-receipt",
					"GET /api/gateway/order/detail",
					"GET /api/gateway/orders",
					"GET /api/gateway/orders/detail",
					"GET /api/gateway/admin/products",
					"GET /api/gateway/admin/products/detail",
					"POST /api/gateway/admin/products/create",
					"POST /api/gateway/admin/products/update",
					"POST /api/gateway/admin/products/stock-adjust",
					"POST /api/gateway/admin/products/card-snapshots/refresh",
					"POST /api/gateway/admin/products/image",
					"GET /api/gateway/admin/suppliers",
					"GET /api/gateway/admin/suppliers/detail",
					"POST /api/gateway/admin/suppliers/create",
					"POST /api/gateway/admin/suppliers/update",
					"GET /api/gateway/admin/promotions",
					"GET /api/gateway/admin/promotions/detail",
					"POST /api/gateway/admin/promotions/create",
					"POST /api/gateway/admin/promotions/update",
					"GET /api/gateway/admin/orders",
					"GET /api/gateway/admin/orders/detail",
					"GET /api/gateway/admin/orders/status-logs",
					"POST /api/gateway/admin/orders/ship",
					"POST /api/gateway/admin/orders/close",
					"POST /api/gateway/admin/orders/refund",
					"GET /api/gateway/admin/refunds",
					"POST /api/gateway/admin/refunds/audit",
					"GET /api/gateway/admin/dashboard/stats",
					"GET /api/gateway/admin/reconciliation/issues",
					"POST /api/gateway/admin/reconciliation/scan",
					"GET /api/gateway/admin/events",
					"POST /api/gateway/admin/events/retry",
					"GET /api/gateway/admin/users",
					"GET /api/gateway/admin/users/detail",
					"POST /api/gateway/admin/users/status",
					"GET /api/gateway/admin/security/events/recent",
					"GET /api/gateway/merchant/me",
					"POST /api/gateway/merchant/apply",
					"GET /api/gateway/merchant/dashboard/stats",
					"GET /api/gateway/merchant/products",
					"POST /api/gateway/merchant/products/create",
					"POST /api/gateway/merchant/products/update",
					"POST /api/gateway/merchant/products/stock-adjust",
					"GET /api/gateway/merchant/orders",
					"POST /api/gateway/merchant/orders/ship",
					"GET /api/gateway/merchant/refunds",
				},
				Notes: "Compatibility routes are kept during branch migration and should not be the final public API names.",
			},
		},
	}
}
