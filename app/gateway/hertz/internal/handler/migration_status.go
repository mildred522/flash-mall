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
