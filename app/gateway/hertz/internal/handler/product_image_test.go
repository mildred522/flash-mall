package handler

import (
	"testing"

	"flash-mall/app/gateway/hertz/internal/config"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestMerchantProductImageUploadRequiresAuthentication(t *testing.T) {
	h := server.Default()
	registerMerchantRoutes(h, &svc.ServiceContext{Config: config.Config{JwtAuthSecret: "jwt-secret"}})

	resp := ut.PerformRequest(h.Engine, "POST", "/api/merchant/products/image", nil).Result()
	if resp.StatusCode() != consts.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", resp.StatusCode(), resp.Body())
	}
}
