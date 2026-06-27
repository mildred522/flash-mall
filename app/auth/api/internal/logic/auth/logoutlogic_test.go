package auth

import (
	"context"
	"testing"

	"flash-mall/app/auth/api/internal/config"
	"flash-mall/app/auth/api/internal/svc"
	"flash-mall/app/auth/api/internal/types"
	"github.com/golang-jwt/jwt/v4"
)

func TestLogoutLogic_Logout_InvalidatesRefreshToken(t *testing.T) {
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

	logout := NewLogoutLogic(context.Background(), svcCtx)
	if err := logout.Logout(loginResp.RefreshToken); err != nil {
		t.Fatalf("logout failed: %v", err)
	}

	refresh := NewRefreshLogic(context.Background(), svcCtx)
	if _, err := refresh.Refresh(loginResp.RefreshToken); err == nil {
		t.Fatalf("expected refresh token to be invalid after logout")
	}
}

func TestLogoutLogic_Logout_InvalidatesRotatedSession(t *testing.T) {
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
	refreshedResp, err := refresh.Refresh(loginResp.RefreshToken)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	logout := NewLogoutLogic(context.Background(), svcCtx)
	if err := logout.Logout(refreshedResp.RefreshToken); err != nil {
		t.Fatalf("logout failed: %v", err)
	}

	if _, err := refresh.Refresh(refreshedResp.RefreshToken); err == nil {
		t.Fatalf("expected rotated refresh token to be invalid after logout")
	}
}

func TestLogoutLogic_LogoutAll_InvalidatesAllUserSessions(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		JwtAuthSecret:          "test-auth-jwt-secret",
		JwtExpireSeconds:       600,
		DemoPassword:           "pwd",
		RefreshTokenTTLSeconds: 3600,
	})

	login := NewLoginPasswordLogic(context.Background(), svcCtx)
	firstResp, err := login.Login(&types.LoginReq{
		UserId:   1001,
		Password: "pwd",
	})
	if err != nil {
		t.Fatalf("first login failed: %v", err)
	}

	secondResp, err := login.Login(&types.LoginReq{
		UserId:   1001,
		Password: "pwd",
	})
	if err != nil {
		t.Fatalf("second login failed: %v", err)
	}

	logoutAll := NewLogoutAllLogic(context.WithValue(context.Background(), "user_id", int64(1001)), svcCtx) //nolint:staticcheck // go-zero JWT stores claims under string keys.
	if err := logoutAll.LogoutAll(); err != nil {
		t.Fatalf("logout-all failed: %v", err)
	}

	refresh := NewRefreshLogic(context.Background(), svcCtx)
	if _, err := refresh.Refresh(firstResp.RefreshToken); err == nil {
		t.Fatalf("expected first refresh token to be invalid after logout-all")
	}
	if _, err := refresh.Refresh(secondResp.RefreshToken); err == nil {
		t.Fatalf("expected second refresh token to be invalid after logout-all")
	}
}

func TestLogoutLogic_LogoutAll_InvalidatesRotatedSession(t *testing.T) {
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
	refreshedResp, err := refresh.Refresh(loginResp.RefreshToken)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	logoutAll := NewLogoutAllLogic(context.WithValue(context.Background(), "user_id", int64(1001)), svcCtx) //nolint:staticcheck // go-zero JWT stores claims under string keys.
	if err := logoutAll.LogoutAll(); err != nil {
		t.Fatalf("logout-all failed: %v", err)
	}

	if _, err := refresh.Refresh(loginResp.RefreshToken); err == nil {
		t.Fatalf("expected old refresh token to be invalid after logout-all")
	}
	if _, err := refresh.Refresh(refreshedResp.RefreshToken); err == nil {
		t.Fatalf("expected rotated refresh token to be invalid after logout-all")
	}
}

func TestLogoutLogic_LogoutAll_BumpsSessionVersionForNextLogin(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		JwtAuthSecret:          "test-auth-jwt-secret",
		JwtExpireSeconds:       600,
		DemoPassword:           "pwd",
		RefreshTokenTTLSeconds: 3600,
	})

	login := NewLoginPasswordLogic(context.Background(), svcCtx)
	if _, err := login.Login(&types.LoginReq{
		UserId:   1001,
		Password: "pwd",
	}); err != nil {
		t.Fatalf("initial login failed: %v", err)
	}

	logoutAll := NewLogoutAllLogic(context.WithValue(context.Background(), "user_id", int64(1001)), svcCtx) //nolint:staticcheck // go-zero JWT stores claims under string keys.
	if err := logoutAll.LogoutAll(); err != nil {
		t.Fatalf("logout-all failed: %v", err)
	}

	nextResp, err := login.Login(&types.LoginReq{
		UserId:   1001,
		Password: "pwd",
	})
	if err != nil {
		t.Fatalf("next login failed: %v", err)
	}

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(nextResp.AccessToken, claims, func(token *jwt.Token) (any, error) {
		return []byte("test-auth-jwt-secret"), nil
	})
	if err != nil || !token.Valid {
		t.Fatalf("parse token failed: %v", err)
	}
	if got := int64(claims["session_version"].(float64)); got != 2 {
		t.Fatalf("unexpected session version: %d", got)
	}
}
