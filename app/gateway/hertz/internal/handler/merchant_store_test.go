package handler

import (
	"context"
	"testing"

	"flash-mall/app/common/authctx"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestMerchantStoreProfileHandlerRequiresIdentity(t *testing.T) {
	c := app.NewContext(0)
	MerchantStoreProfileHandler(&svc.ServiceContext{})(context.Background(), c)
	if c.Response.StatusCode() != consts.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", c.Response.StatusCode(), c.Response.Body())
	}
}

func TestMerchantStoreUpdateHandlerRejectsInvalidRequestBeforeDatabase(t *testing.T) {
	c := app.NewContext(0)
	c.Request.SetBodyString(`{"logo_url":"javascript:alert(1)","expected_version":0}`)
	ctx := authctx.WithIdentity(context.Background(), authctx.Identity{UserID: 1001, Role: authctx.RoleMerchant})
	MerchantStoreUpdateHandler(&svc.ServiceContext{})(ctx, c)
	if c.Response.StatusCode() != consts.StatusBadRequest {
		t.Fatalf("status=%d body=%s", c.Response.StatusCode(), c.Response.Body())
	}
}
