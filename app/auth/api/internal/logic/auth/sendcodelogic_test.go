package auth

import (
	"context"
	"testing"

	"flash-mall/app/auth/api/internal/config"
	"flash-mall/app/auth/api/internal/svc"
	"flash-mall/app/auth/api/internal/types"
)

func TestSendCodeLogic_Send_Success(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		JwtAuthSecret:          "test-auth-jwt-secret",
		JwtExpireSeconds:       600,
		DemoPassword:           "pwd",
		RefreshTokenTTLSeconds: 3600,
		CodeTTLSeconds:         300,
	})

	l := NewSendCodeLogic(context.Background(), svcCtx)
	resp, err := l.Send(&types.SendCodeReq{
		Phone: "13800138000",
		Scene: "register",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Sent {
		t.Fatalf("expected sent=true")
	}
	if resp.DebugCode == "" {
		t.Fatalf("expected debug code")
	}
	if resp.ExpiresAt <= 0 {
		t.Fatalf("expected expires_at")
	}
}
