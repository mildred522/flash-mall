package logic

import (
	"context"
	"testing"

	"flash-mall/app/order/api/internal/config"
	"flash-mall/app/order/api/internal/svc"
	"flash-mall/app/order/api/internal/types"
)

func TestLoginLogic_Login_Success(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			JwtAuthSecret:    "test-jwt-secret",
			JwtExpireSeconds: 600,
			AuthDemoPassword: "pwd",
		},
	}

	l := NewLoginLogic(context.Background(), svcCtx)
	resp, err := l.Login(&types.LoginReq{
		UserId:   1001,
		Password: "pwd",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.AccessToken == "" {
		t.Fatalf("empty access token")
	}
	if resp.TokenType != "Bearer" {
		t.Fatalf("unexpected token type: %s", resp.TokenType)
	}
	if resp.ExpiresAt <= 0 {
		t.Fatalf("invalid expires_at: %d", resp.ExpiresAt)
	}
}

func TestLoginLogic_Login_BadPassword(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			JwtAuthSecret:    "test-jwt-secret",
			JwtExpireSeconds: 600,
			AuthDemoPassword: "pwd",
		},
	}

	l := NewLoginLogic(context.Background(), svcCtx)
	_, err := l.Login(&types.LoginReq{
		UserId:   1001,
		Password: "wrong",
	})
	if err == nil {
		t.Fatalf("expected auth error")
	}
}
