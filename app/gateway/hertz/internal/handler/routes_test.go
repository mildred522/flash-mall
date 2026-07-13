package handler

import (
	"testing"
	"time"

	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app/server"
)

func TestInventoryRoutesExposeReadOnlySummary(t *testing.T) {
	h := server.Default()
	registerInventoryRoutes(h, &svc.ServiceContext{})

	for _, route := range h.Routes() {
		if route.Path == "/api/inventory/reserve" || route.Path == "/api/inventory/release" || route.Path == "/api/inventory/confirm-deduct" {
			t.Fatalf("inventory mutation route must not be public: %s %s", route.Method, route.Path)
		}
	}
}

func TestShopRoutesExposeBundledProductAssets(t *testing.T) {
	h := server.Default()
	registerShopRoutes(h, &svc.ServiceContext{})

	for _, route := range h.Routes() {
		if route.Method == "GET" && route.Path == "/products/*any" {
			return
		}
	}
	t.Fatal("GET /products/*any route is missing")
}

func TestPaymentRoutesExposeIntentStatusAndSandboxConfirmation(t *testing.T) {
	h := server.Default()
	svcCtx := &svc.ServiceContext{}
	registerSystemRoutes(h, svcCtx, time.Now())
	registerAuthRoutes(h, svcCtx)
	registerOrderRoutes(h, svcCtx)

	want := map[string]string{
		"GET /pay":                          "",
		"GET /api/payment/status":           "",
		"POST /api/payment/sandbox/confirm": "",
		"POST /api/order/pay":               "",
	}
	for _, route := range h.Routes() {
		delete(want, route.Method+" "+route.Path)
	}
	for route := range want {
		t.Errorf("payment route is missing: %s", route)
	}
}

func TestMerchantPageAndImageUploadRoutesAreIndependent(t *testing.T) {
	h := server.Default()
	svcCtx := &svc.ServiceContext{}
	registerSystemRoutes(h, svcCtx, time.Now())
	registerMerchantRoutes(h, svcCtx)

	want := map[string]bool{
		"GET /merchant":                     false,
		"GET /merchant/*any":                false,
		"GET /api/merchant/application":     false,
		"POST /api/merchant/products/image": false,
	}
	for _, route := range h.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for route, found := range want {
		if !found {
			t.Errorf("merchant route is missing: %s", route)
		}
	}
}
