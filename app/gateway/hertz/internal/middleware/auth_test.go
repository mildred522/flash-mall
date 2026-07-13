package middleware

import (
	"context"
	"fmt"
	"testing"
	"time"

	"flash-mall/app/common/authctx"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/golang-jwt/jwt/v4"
)

func TestOptionalIdentityAllowsAnonymousRequest(t *testing.T) {
	h := server.Default()
	h.GET("/optional", OptionalIdentity("secret"), func(ctx context.Context, c *app.RequestContext) {
		if _, found := authctx.IdentityFrom(ctx); found {
			t.Fatal("anonymous request should not have an identity")
		}
		c.Status(consts.StatusNoContent)
	})

	resp := ut.PerformRequest(h.Engine, "GET", "/optional", nil).Result()
	if resp.StatusCode() != consts.StatusNoContent {
		t.Fatalf("status=%d body=%s", resp.StatusCode(), resp.Body())
	}
}

func TestOptionalIdentityInjectsValidBearer(t *testing.T) {
	secret := "secret"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"user_id": 42, "role": "user", "exp": time.Now().Add(time.Hour).Unix()})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	h := server.Default()
	h.GET("/optional", OptionalIdentity(secret), func(ctx context.Context, c *app.RequestContext) {
		identity, found := authctx.IdentityFrom(ctx)
		if !found {
			t.Fatal("valid bearer should inject identity")
		}
		c.String(consts.StatusOK, fmt.Sprint(identity.UserID))
	})

	resp := ut.PerformRequest(h.Engine, "GET", "/optional", nil, ut.Header{Key: "Authorization", Value: "Bearer " + signed}).Result()
	if resp.StatusCode() != consts.StatusOK || string(resp.Body()) != "42" {
		t.Fatalf("status=%d body=%s", resp.StatusCode(), resp.Body())
	}
}

func TestOptionalIdentityRejectsInvalidSuppliedBearer(t *testing.T) {
	h := server.Default()
	h.GET("/optional", OptionalIdentity("secret"), func(_ context.Context, c *app.RequestContext) {
		c.Status(consts.StatusNoContent)
	})

	resp := ut.PerformRequest(h.Engine, "GET", "/optional", nil, ut.Header{Key: "Authorization", Value: "Bearer invalid"}).Result()
	if resp.StatusCode() != consts.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", resp.StatusCode(), resp.Body())
	}
}
