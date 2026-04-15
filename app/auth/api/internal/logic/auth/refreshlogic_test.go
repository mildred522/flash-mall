package auth

import (
	"context"
	"testing"

	"flash-mall/app/auth/api/internal/config"
	"flash-mall/app/auth/api/internal/svc"
	"flash-mall/app/auth/api/internal/types"
)

func TestRefreshLogic_Refresh_RotatesToken(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		JwtAuthSecret:          "test-auth-jwt-secret",
		JwtExpireSeconds:       600,
		DemoPassword:           "pwd",
		RefreshTokenTTLSeconds: 3600,
	})

	login := NewLoginPasswordLogic(context.Background(), svcCtx)
	loginResp, err := login.Login(&types.LoginReq{
		UserId:   1001,
		Password: "pwd",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	refresh := NewRefreshLogic(context.Background(), svcCtx)
	refreshResp, err := refresh.Refresh(loginResp.RefreshToken)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if refreshResp.AccessToken == "" {
		t.Fatalf("expected access token")
	}
	if refreshResp.RefreshToken == "" {
		t.Fatalf("expected refresh token")
	}
	if refreshResp.RefreshToken == loginResp.RefreshToken {
		t.Fatalf("expected rotated refresh token")
	}
}
