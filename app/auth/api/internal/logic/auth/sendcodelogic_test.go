package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"flash-mall/app/auth/api/internal/config"
	"flash-mall/app/auth/api/internal/risk"
	"flash-mall/app/auth/api/internal/svc"
	"flash-mall/app/auth/api/internal/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func TestSendCodeLogic_Send_ThrottlesByCooldown(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		JwtAuthSecret:          "test-auth-jwt-secret",
		JwtExpireSeconds:       600,
		DemoPassword:           "pwd",
		RefreshTokenTTLSeconds: 3600,
		CodeTTLSeconds:         300,
	})

	l := NewSendCodeLogic(context.Background(), svcCtx)
	req := &types.SendCodeReq{
		Phone:    "13800138000",
		Scene:    "register",
		ClientIP: "203.0.113.7",
	}
	if _, err := l.Send(req); err != nil {
		t.Fatalf("first send failed: %v", err)
	}
	if _, err := l.Send(req); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("expected cooldown throttle, got %v", err)
	}
}

func TestSendCodeLogic_Send_ThrottlesByPhoneWindow(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		JwtAuthSecret:              "test-auth-jwt-secret",
		JwtExpireSeconds:           600,
		DemoPassword:               "pwd",
		RefreshTokenTTLSeconds:     3600,
		CodeTTLSeconds:             300,
		CodeSendCooldownSeconds:    1,
		CodeSendPhoneWindowSeconds: 60,
		CodeSendPhoneMaxAttempts:   2,
		CodeSendIPWindowSeconds:    60,
		CodeSendIPMaxAttempts:      10,
	})

	limiter := svcCtx.RiskLimiter.(*risk.MemoryLimiter)
	key := "auth:risk:code:phone:register:13800138000:window"
	ttl := time.Duration(svcCtx.Config.CodeSendPhoneWindowSeconds) * time.Second
	if err := limiter.Incr(context.Background(), key, ttl); err != nil {
		t.Fatalf("seed first phone-window attempt: %v", err)
	}
	if err := limiter.Incr(context.Background(), key, ttl); err != nil {
		t.Fatalf("seed second phone-window attempt: %v", err)
	}

	l := NewSendCodeLogic(context.Background(), svcCtx)
	_, err := l.Send(&types.SendCodeReq{
		Phone:    "13800138000",
		Scene:    "register",
		ClientIP: "203.0.113.7",
	})
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("expected phone-window throttle, got %v", err)
	}
}

func TestSendCodeLogic_Send_ThrottlesByIPWindow(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		JwtAuthSecret:              "test-auth-jwt-secret",
		JwtExpireSeconds:           600,
		DemoPassword:               "pwd",
		RefreshTokenTTLSeconds:     3600,
		CodeTTLSeconds:             300,
		CodeSendCooldownSeconds:    1,
		CodeSendPhoneWindowSeconds: 60,
		CodeSendPhoneMaxAttempts:   10,
		CodeSendIPWindowSeconds:    60,
		CodeSendIPMaxAttempts:      2,
	})

	limiter := svcCtx.RiskLimiter.(*risk.MemoryLimiter)
	key := "auth:risk:code:ip:register:203.0.113.7"
	ttl := time.Duration(svcCtx.Config.CodeSendIPWindowSeconds) * time.Second
	if err := limiter.Incr(context.Background(), key, ttl); err != nil {
		t.Fatalf("seed first ip-window attempt: %v", err)
	}
	if err := limiter.Incr(context.Background(), key, ttl); err != nil {
		t.Fatalf("seed second ip-window attempt: %v", err)
	}

	l := NewSendCodeLogic(context.Background(), svcCtx)
	_, err := l.Send(&types.SendCodeReq{
		Phone:    "13800138001",
		Scene:    "register",
		ClientIP: "203.0.113.7",
	})
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("expected ip-window throttle, got %v", err)
	}
}

func TestSendCodeLogic_Send_StillReturnsCodeWhenRiskPersistenceFails(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		JwtAuthSecret:              "test-auth-jwt-secret",
		JwtExpireSeconds:           600,
		DemoPassword:               "pwd",
		RefreshTokenTTLSeconds:     3600,
		CodeTTLSeconds:             300,
		CodeSendCooldownSeconds:    1,
		CodeSendPhoneWindowSeconds: 60,
		CodeSendPhoneMaxAttempts:   3,
		CodeSendIPWindowSeconds:    60,
		CodeSendIPMaxAttempts:      3,
	})
	svcCtx.RiskLimiter = failingPersistLimiter{}

	l := NewSendCodeLogic(context.Background(), svcCtx)
	req := &types.SendCodeReq{
		Phone:    "13800138000",
		Scene:    "register",
		ClientIP: "203.0.113.7",
	}
	resp, err := l.Send(req)
	if err != nil {
		t.Fatalf("expected send to succeed even if risk persistence fails, got resp=%v err=%v", resp, err)
	}
	if resp == nil || resp.DebugCode == "" {
		t.Fatalf("expected issued code to be returned, got %#v", resp)
	}
	if err := svcCtx.Store.ConsumeCode("13800138000", "register", resp.DebugCode, 3); err != nil {
		t.Fatalf("expected issued code to remain usable, got %v", err)
	}
}

func TestSendCodeLogic_Send_RecordsAuditEvents(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		JwtAuthSecret:          "test-auth-jwt-secret",
		JwtExpireSeconds:       600,
		DemoPassword:           "pwd",
		RefreshTokenTTLSeconds: 3600,
		CodeTTLSeconds:         300,
	})

	l := NewSendCodeLogic(context.Background(), svcCtx)
	req := &types.SendCodeReq{
		Phone:     "13800138000",
		Scene:     "register",
		ClientIP:  "203.0.113.7",
		UserAgent: "audit-test",
	}
	if _, err := l.Send(req); err != nil {
		t.Fatalf("first send failed: %v", err)
	}
	if _, err := l.Send(req); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("expected cooldown throttle, got %v", err)
	}

	events, err := svcCtx.AuditRecorder.ListRecent(context.Background(), 10)
	if err != nil {
		t.Fatalf("list audit events: %v", err)
	}
	if len(events) < 2 {
		t.Fatalf("expected send-code audit events, got %#v", events)
	}
	if events[0].EventType != "send_code_blocked" || events[0].Result != "blocked" {
		t.Fatalf("unexpected newest event: %#v", events[0])
	}
	if events[1].EventType != "send_code_success" || events[1].Result != "success" {
		t.Fatalf("unexpected previous event: %#v", events[1])
	}
	if events[0].IdentityValue != "13800138000" || events[1].IdentityValue != "13800138000" {
		t.Fatalf("unexpected subject in audit events: %#v", events)
	}
}

type failingPersistLimiter struct{}

func (failingPersistLimiter) Incr(context.Context, string, time.Duration) error {
	return errors.New("persist failed")
}

func (failingPersistLimiter) Blocked(context.Context, string, int64) (bool, int64, error) {
	return false, 0, nil
}

func (failingPersistLimiter) Reset(context.Context, string) error {
	return nil
}
