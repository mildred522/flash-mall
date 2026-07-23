package handler

import (
	"strings"
	"testing"
)

func TestRouteMigrationStatusIncludesStorefrontShowcaseMetricsAndArchitectureBoundaries(t *testing.T) {
	status := routeMigrationStatus()
	var all strings.Builder
	for _, group := range status.Groups {
		all.WriteString(strings.Join(group.LiveRoutes, "\n"))
		all.WriteString("\n")
		all.WriteString(group.Notes)
		all.WriteString("\n")
	}
	text := all.String()
	for _, want := range []string{
		"GET /metrics",
		"GET /product/*any",
		"GET /store/*any",
		"GET /api/shop/catalog",
		"GET /api/shop/products/detail",
		"GET /api/shop/stores/detail",
		"GET /api/shop/stores/products",
		"GET /api/admin/showcase",
		"GET /api/admin/showcase/candidates",
		"POST /api/admin/showcase/publish",
		"GET /api/merchant/store/profile",
		"POST /api/merchant/store/profile",
		"POST /api/merchant/store/assets",
		"go-zero product-rpc",
		"Kitex inventory commands",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("migration status is missing %q", want)
		}
	}
}

func TestRouteMigrationStatusDoesNotPlanAlreadyLiveRoutes(t *testing.T) {
	status := routeMigrationStatus()
	live := make(map[string]struct{})
	for _, group := range status.Groups {
		for _, route := range group.LiveRoutes {
			live[route] = struct{}{}
		}
	}
	for _, group := range status.Groups {
		for _, route := range group.PlannedRoutes {
			if _, exists := live[route]; exists {
				t.Errorf("route %q is both live and planned", route)
			}
		}
	}
	for _, route := range []string{
		"GET /api/admin/campaigns",
		"POST /api/admin/campaigns/upsert",
		"GET /api/merchant/inventory/stock-changes",
		"GET /api/payment/status",
		"POST /api/payment/sandbox/confirm",
	} {
		if _, exists := live[route]; !exists {
			t.Errorf("registered Hertz route is missing from live migration status: %s", route)
		}
	}
}

func TestRouteMigrationStatusAssignsEachLiveRouteToOneGroup(t *testing.T) {
	owners := make(map[string]string)
	for _, group := range routeMigrationStatus().Groups {
		for _, route := range group.LiveRoutes {
			if owner, exists := owners[route]; exists {
				t.Errorf("route %q belongs to both %q and %q", route, owner, group.Name)
			}
			owners[route] = group.Name
		}
	}
}
