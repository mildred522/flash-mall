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
