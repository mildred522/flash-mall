package handler

import (
	"fmt"
	"testing"

	"flash-mall/app/entry/api/internal/config"
	"flash-mall/app/entry/api/internal/svc"

	"github.com/zeromicro/go-zero/rest"
)

func TestRegisterHandlersHasNoDuplicateRoutes(t *testing.T) {
	server := rest.MustNewServer(rest.RestConf{
		Host: "127.0.0.1",
		Port: 0,
	})
	RegisterHandlers(server, &svc.ServiceContext{
		Config: config.Config{
			JwtAuthSecret: "test-secret",
		},
	})

	seen := make(map[string]struct{})
	for _, route := range server.Routes() {
		key := fmt.Sprintf("%s %s", route.Method, route.Path)
		if _, ok := seen[key]; ok {
			t.Fatalf("duplicate route registered: %s", key)
		}
		seen[key] = struct{}{}
	}
}

func TestRegisterHandlersIncludesPublicPaymentCallbackRoute(t *testing.T) {
	server := rest.MustNewServer(rest.RestConf{
		Host: "127.0.0.1",
		Port: 0,
	})
	RegisterHandlers(server, &svc.ServiceContext{
		Config: config.Config{
			JwtAuthSecret: "test-secret",
		},
	})

	for _, route := range server.Routes() {
		if route.Method == "POST" && route.Path == "/api/payment/callback" {
			return
		}
	}
	t.Fatal("POST /api/payment/callback route is not registered")
}
