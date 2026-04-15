package auth

import (
	"context"
	"testing"

	"flash-mall/app/auth/api/internal/config"
	"flash-mall/app/auth/api/internal/svc"
	"flash-mall/app/auth/api/internal/types"
)

func TestRegisterLogic_Register_Success(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		JwtAuthSecret:          "test-auth-jwt-secret",
		JwtExpireSeconds:       600,
		DemoPassword:           "pwd",
		RefreshTokenTTLSeconds: 3600,
		CodeTTLSeconds:         300,
	})

	sendCode := NewSendCodeLogic(context.Background(), svcCtx)
	codeResp, err := sendCode.Send(&types.SendCodeReq{
		Phone: "13800138000",
		Scene: "register",
	})
	if err != nil {
		t.Fatalf("send code failed: %v", err)
	}

	l := NewRegisterLogic(context.Background(), svcCtx)
	resp, err := l.Register(&types.RegisterReq{
		Phone:       "13800138000",
		Password:    "mall-pass-123",
		Code:        codeResp.DebugCode,
		DisplayName: "测试新用户",
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

	login := NewLoginPasswordLogic(context.Background(), svcCtx)
	loginResp, err := login.Login(&types.LoginReq{
		Phone:    "13800138000",
		Password: "mall-pass-123",
	})
	if err != nil {
		t.Fatalf("phone login failed: %v", err)
	}
	if loginResp.AccessToken == "" {
		t.Fatalf("expected access token after register")
	}
}
