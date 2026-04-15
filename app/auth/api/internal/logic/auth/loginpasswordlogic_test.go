package auth

import (
	"context"
	"testing"

	"flash-mall/app/auth/api/internal/config"
	"flash-mall/app/auth/api/internal/svc"
	"flash-mall/app/auth/api/internal/types"
)

func TestLoginPasswordLogic_Login_Success(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		JwtAuthSecret:          "test-auth-jwt-secret",
		JwtExpireSeconds:       600,
		DemoPassword:           "pwd",
		RefreshTokenTTLSeconds: 3600,
	})

	l := NewLoginPasswordLogic(context.Background(), svcCtx)
	resp, err := l.Login(&types.LoginReq{
		UserId:   1001,
		Password: "pwd",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.AccessToken == "" {
		t.Fatalf("expected non-empty access token")
	}
	if resp.TokenType != "Bearer" {
		t.Fatalf("unexpected token type: %s", resp.TokenType)
	}
	if resp.ExpiresAt <= 0 {
		t.Fatalf("invalid expires_at: %d", resp.ExpiresAt)
	}
	if resp.RefreshToken == "" {
		t.Fatalf("expected refresh token")
	}
}

func TestLoginPasswordLogic_Login_ByPhone(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		JwtAuthSecret:          "test-auth-jwt-secret",
		JwtExpireSeconds:       600,
		DemoPassword:           "pwd",
		RefreshTokenTTLSeconds: 3600,
	})

	l := NewLoginPasswordLogic(context.Background(), svcCtx)
	resp, err := l.Login(&types.LoginReq{
		Phone:    "13800000001",
		Password: "pwd",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.AccessToken == "" {
		t.Fatalf("expected access token")
	}
	if resp.RefreshToken == "" {
		t.Fatalf("expected refresh token")
	}
}

func TestLoginPasswordLogic_Login_BadPassword(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		JwtAuthSecret:          "test-auth-jwt-secret",
		JwtExpireSeconds:       600,
		DemoPassword:           "pwd",
		RefreshTokenTTLSeconds: 3600,
	})

	l := NewLoginPasswordLogic(context.Background(), svcCtx)
	_, err := l.Login(&types.LoginReq{
		UserId:   1001,
		Password: "wrong",
	})
	if err == nil {
		t.Fatalf("expected auth error")
	}
}

func TestLoginPasswordLogic_Login_SameDeviceReplacesOldSession(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		JwtAuthSecret:          "test-auth-jwt-secret",
		JwtExpireSeconds:       600,
		DemoPassword:           "pwd",
		RefreshTokenTTLSeconds: 3600,
	})

	l := NewLoginPasswordLogic(context.Background(), svcCtx)
	firstResp, err := l.Login(&types.LoginReq{
		UserId:     1001,
		Password:   "pwd",
		DeviceType: "web",
	})
	if err != nil {
		t.Fatalf("first login failed: %v", err)
	}
	secondResp, err := l.Login(&types.LoginReq{
		UserId:     1001,
		Password:   "pwd",
		DeviceType: "web",
	})
	if err != nil {
		t.Fatalf("second login failed: %v", err)
	}

	refresh := NewRefreshLogic(context.Background(), svcCtx)
	if _, err := refresh.Refresh(firstResp.RefreshToken); err == nil {
		t.Fatalf("expected first same-device refresh token to be invalid")
	}
	if _, err := refresh.Refresh(secondResp.RefreshToken); err != nil {
		t.Fatalf("expected latest same-device refresh token to stay active: %v", err)
	}
}

func TestLoginPasswordLogic_Login_DifferentDevicesCanCoexist(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		JwtAuthSecret:          "test-auth-jwt-secret",
		JwtExpireSeconds:       600,
		DemoPassword:           "pwd",
		RefreshTokenTTLSeconds: 3600,
	})

	l := NewLoginPasswordLogic(context.Background(), svcCtx)
	webResp, err := l.Login(&types.LoginReq{
		UserId:     1001,
		Password:   "pwd",
		DeviceType: "web",
	})
	if err != nil {
		t.Fatalf("web login failed: %v", err)
	}
	iosResp, err := l.Login(&types.LoginReq{
		UserId:     1001,
		Password:   "pwd",
		DeviceType: "ios",
	})
	if err != nil {
		t.Fatalf("ios login failed: %v", err)
	}

	refresh := NewRefreshLogic(context.Background(), svcCtx)
	if _, err := refresh.Refresh(webResp.RefreshToken); err != nil {
		t.Fatalf("expected web refresh token to remain active: %v", err)
	}
	if _, err := refresh.Refresh(iosResp.RefreshToken); err != nil {
		t.Fatalf("expected ios refresh token to remain active: %v", err)
	}
}
