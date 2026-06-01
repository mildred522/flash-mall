package auth

import (
	"context"
	"testing"

	"flash-mall/app/auth/api/internal/audit"
	"flash-mall/app/auth/api/internal/config"
	"flash-mall/app/auth/api/internal/risk"
	"flash-mall/app/auth/api/internal/svc"
	"flash-mall/app/auth/api/internal/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func TestLoginPasswordLogic_Login_ThrottlesByPhone(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		JwtAuthSecret:             "test-auth-jwt-secret",
		JwtExpireSeconds:          600,
		DemoPassword:              "pwd",
		RefreshTokenTTLSeconds:    3600,
		LoginFailWindowSeconds:    60,
		LoginFailPhoneMaxAttempts: 2,
		LoginFailIPMaxAttempts:    10,
	})

	l := NewLoginPasswordLogic(context.Background(), svcCtx)
	req := &types.LoginReq{
		Phone:    "13800000001",
		Password: "wrong",
		ClientIP: "203.0.113.9",
	}
	for i := 0; i < 2; i++ {
		if _, err := l.Login(req); status.Code(err) != codes.Unauthenticated {
			t.Fatalf("expected unauthenticated on failed attempt %d, got %v", i+1, err)
		}
	}
	if _, err := l.Login(req); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("expected throttled login after phone limit, got %v", err)
	}
}

func TestLoginPasswordLogic_Login_ThrottlesByIP(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		JwtAuthSecret:             "test-auth-jwt-secret",
		JwtExpireSeconds:          600,
		DemoPassword:              "pwd",
		RefreshTokenTTLSeconds:    3600,
		LoginFailWindowSeconds:    60,
		LoginFailPhoneMaxAttempts: 10,
		LoginFailIPMaxAttempts:    2,
	})

	l := NewLoginPasswordLogic(context.Background(), svcCtx)
	req1 := &types.LoginReq{
		Phone:    "13800000001",
		Password: "wrong",
		ClientIP: "203.0.113.9",
	}
	req2 := &types.LoginReq{
		Phone:    "13800000002",
		Password: "wrong",
		ClientIP: "203.0.113.9",
	}
	if _, err := l.Login(req1); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected unauthenticated on first failed attempt, got %v", err)
	}
	if _, err := l.Login(req2); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected unauthenticated on second failed attempt, got %v", err)
	}
	if _, err := l.Login(req1); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("expected throttled login after IP limit, got %v", err)
	}
}

func TestLoginPasswordLogic_Login_ResetsCountersOnSuccess(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		JwtAuthSecret:             "test-auth-jwt-secret",
		JwtExpireSeconds:          600,
		DemoPassword:              "pwd",
		RefreshTokenTTLSeconds:    3600,
		LoginFailWindowSeconds:    60,
		LoginFailPhoneMaxAttempts: 2,
		LoginFailIPMaxAttempts:    2,
	})

	l := NewLoginPasswordLogic(context.Background(), svcCtx)
	_, err := l.Login(&types.LoginReq{
		Phone:    "13800000001",
		Password: "wrong",
		ClientIP: "203.0.113.9",
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected unauthenticated on failed attempt, got %v", err)
	}
	if _, err := l.Login(&types.LoginReq{
		Phone:    "13800000001",
		Password: "pwd",
		ClientIP: "203.0.113.9",
	}); err != nil {
		t.Fatalf("expected successful login to reset counters, got %v", err)
	}
	if _, err := l.Login(&types.LoginReq{
		Phone:    "13800000001",
		Password: "wrong",
		ClientIP: "203.0.113.9",
	}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected counters to be reset after success, got %v", err)
	}
}

func TestLoginPasswordLogic_Login_DoesNotResetSharedIPBucketOnSuccess(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		JwtAuthSecret:             "test-auth-jwt-secret",
		JwtExpireSeconds:          600,
		DemoPassword:              "pwd",
		RefreshTokenTTLSeconds:    3600,
		LoginFailWindowSeconds:    60,
		LoginFailPhoneMaxAttempts: 10,
		LoginFailIPMaxAttempts:    2,
	})
	if _, err := svcCtx.Store.CreateUser("13800000003", "User 3", "pwd3"); err != nil {
		t.Fatalf("create second user: %v", err)
	}

	l := NewLoginPasswordLogic(context.Background(), svcCtx)
	ip := "203.0.113.9"

	if _, err := l.Login(&types.LoginReq{
		Phone:    "13800000001",
		Password: "wrong",
		ClientIP: ip,
	}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected first failure to be unauthenticated, got %v", err)
	}
	if _, err := l.Login(&types.LoginReq{
		Phone:    "13800000003",
		Password: "pwd3",
		ClientIP: ip,
	}); err != nil {
		t.Fatalf("expected successful login for second user, got %v", err)
	}
	if _, err := l.Login(&types.LoginReq{
		Phone:    "13800000001",
		Password: "wrong",
		ClientIP: ip,
	}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected second failure after success to be unauthenticated, got %v", err)
	}
	if _, err := l.Login(&types.LoginReq{
		Phone:    "13800000001",
		Password: "wrong",
		ClientIP: ip,
	}); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("expected shared IP bucket to remain enforced, got %v", err)
	}
}

func TestLoginPasswordLogic_Login_ThrottlesUserIDPath(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		JwtAuthSecret:             "test-auth-jwt-secret",
		JwtExpireSeconds:          600,
		DemoPassword:              "pwd",
		RefreshTokenTTLSeconds:    3600,
		LoginFailWindowSeconds:    60,
		LoginFailPhoneMaxAttempts: 2,
		LoginFailIPMaxAttempts:    10,
	})
	limiter := svcCtx.RiskLimiter.(*risk.MemoryLimiter)
	key := "auth:risk:login:user:1001"

	l := NewLoginPasswordLogic(context.Background(), svcCtx)
	req := &types.LoginReq{
		UserId:   1001,
		Password: "wrong",
		ClientIP: "203.0.113.9",
	}
	for i := 0; i < 2; i++ {
		if _, err := l.Login(req); status.Code(err) != codes.Unauthenticated {
			t.Fatalf("expected unauthenticated on failed user-id attempt %d, got %v", i+1, err)
		}
	}
	blocked, count, err := limiter.Blocked(context.Background(), key, 2)
	if err != nil {
		t.Fatalf("check user-id bucket: %v", err)
	}
	if !blocked || count != 2 {
		t.Fatalf("expected user-id bucket to be tracked, blocked=%v count=%d", blocked, count)
	}
	if _, err := l.Login(req); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("expected user-id login to be throttled, got %v", err)
	}
}

func TestLoginPasswordLogic_Login_RecordsAuditEvents(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		JwtAuthSecret:          "test-auth-jwt-secret",
		JwtExpireSeconds:       600,
		DemoPassword:           "pwd",
		RefreshTokenTTLSeconds: 3600,
	})

	l := NewLoginPasswordLogic(context.Background(), svcCtx)
	if _, err := l.Login(&types.LoginReq{
		Phone:     "13800000001",
		Password:  "wrong",
		ClientIP:  "203.0.113.9",
		UserAgent: "audit-test",
	}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected failed login to be unauthenticated, got %v", err)
	}
	if _, err := l.Login(&types.LoginReq{
		Phone:     "13800000001",
		Password:  "pwd",
		ClientIP:  "203.0.113.9",
		UserAgent: "audit-test",
	}); err != nil {
		t.Fatalf("expected successful login, got %v", err)
	}

	events, err := svcCtx.AuditRecorder.ListRecent(context.Background(), 10)
	if err != nil {
		t.Fatalf("list audit events: %v", err)
	}
	if len(events) < 2 {
		t.Fatalf("expected login audit events, got %#v", events)
	}
	if events[0].EventType != "login_password_success" || events[0].Result != "success" {
		t.Fatalf("unexpected newest event: %#v", events[0])
	}
	if events[0].UserID != 1001 || events[0].IdentityValue != "13800000001" || events[0].IP != "203.0.113.9" {
		t.Fatalf("unexpected success audit payload: %#v", events[0])
	}
	if events[1].EventType != "login_password_fail" || events[1].Result != "fail" {
		t.Fatalf("unexpected previous event: %#v", events[1])
	}
	if events[1].UserID != 1001 || events[1].IdentityValue != "13800000001" || events[1].IP != "203.0.113.9" {
		t.Fatalf("unexpected failure audit payload: %#v", events[1])
	}
	if events[0].CreatedAt.IsZero() || events[1].CreatedAt.IsZero() {
		t.Fatalf("expected recorder to stamp CreatedAt, got %#v", events)
	}
	if _, ok := svcCtx.AuditRecorder.(*audit.MemoryRecorder); !ok {
		t.Fatalf("expected test recorder to be memory-backed, got %T", svcCtx.AuditRecorder)
	}
}
