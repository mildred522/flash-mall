package auth

import (
	"context"
	"testing"

	"flash-mall/app/auth/api/internal/config"
	"flash-mall/app/auth/api/internal/svc"
	"flash-mall/app/auth/api/internal/types"
)

func TestResetPasswordLogic_ResetPassword_UpdatesCredentialAndInvalidatesSessions(t *testing.T) {
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
		Scene: "reset-password",
	})
	if err != nil {
		t.Fatalf("send code failed: %v", err)
	}

	login := NewLoginPasswordLogic(context.Background(), svcCtx)
	loginResp, err := login.Login(&types.LoginReq{
		Phone:    "13800000001",
		Password: "pwd",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	reset := NewResetPasswordLogic(context.Background(), svcCtx)
	if err := reset.Reset(&types.ResetPasswordReq{
		Phone:       "13800000001",
		Code:        codeResp.DebugCode,
		NewPassword: "new-pass-456",
	}); err != nil {
		t.Fatalf("reset password failed: %v", err)
	}

	if _, err := login.Login(&types.LoginReq{
		Phone:    "13800000001",
		Password: "pwd",
	}); err == nil {
		t.Fatalf("expected old password to be invalid")
	}

	if _, err := login.Login(&types.LoginReq{
		Phone:    "13800000001",
		Password: "new-pass-456",
	}); err != nil {
		t.Fatalf("expected new password to login successfully: %v", err)
	}

	refresh := NewRefreshLogic(context.Background(), svcCtx)
	if _, err := refresh.Refresh(loginResp.RefreshToken); err == nil {
		t.Fatalf("expected refresh token to be invalid after password reset")
	}
}

func TestResetPasswordLogic_ResetPassword_RevokesRefreshedSessions(t *testing.T) {
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
		Scene: "reset-password",
	})
	if err != nil {
		t.Fatalf("send code failed: %v", err)
	}

	login := NewLoginPasswordLogic(context.Background(), svcCtx)
	loginResp, err := login.Login(&types.LoginReq{
		Phone:    "13800000001",
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

	reset := NewResetPasswordLogic(context.Background(), svcCtx)
	if err := reset.Reset(&types.ResetPasswordReq{
		Phone:       "13800000001",
		Code:        codeResp.DebugCode,
		NewPassword: "new-pass-456",
	}); err != nil {
		t.Fatalf("reset password failed: %v", err)
	}

	if _, err := refresh.Refresh(loginResp.RefreshToken); err == nil {
		t.Fatalf("expected old refresh token to be invalid after password reset")
	}
	if _, err := refresh.Refresh(refreshedResp.RefreshToken); err == nil {
		t.Fatalf("expected rotated refresh token to be invalid after password reset")
	}
}
