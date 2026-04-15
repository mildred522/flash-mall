package auth

import (
	"context"
	"testing"

	"flash-mall/app/auth/api/internal/config"
	"flash-mall/app/auth/api/internal/svc"
	"flash-mall/app/auth/api/internal/types"
)

func TestLoginCodeLogic_Login_Success(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		JwtAuthSecret:          "test-auth-jwt-secret",
		JwtExpireSeconds:       600,
		DemoPassword:           "pwd",
		RefreshTokenTTLSeconds: 3600,
		CodeTTLSeconds:         300,
	})

	sendCode := NewSendCodeLogic(context.Background(), svcCtx)
	codeResp, err := sendCode.Send(&types.SendCodeReq{
		Phone: "13800000001",
		Scene: "login",
	})
	if err != nil {
		t.Fatalf("send code failed: %v", err)
	}

	login := NewLoginCodeLogic(context.Background(), svcCtx)
	resp, err := login.Login(&types.LoginCodeReq{
		Phone: "13800000001",
		Code:  codeResp.DebugCode,
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
