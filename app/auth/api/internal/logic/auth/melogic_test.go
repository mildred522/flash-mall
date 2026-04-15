package auth

import (
	"context"
	"testing"

	"flash-mall/app/auth/api/internal/config"
	"flash-mall/app/auth/api/internal/svc"
)

func TestMeLogic_Me_Success(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		DemoPassword: "pwd",
	})
	sessionID, _, err := svcCtx.Store.CreateSession(1001, 3600)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	ctx := context.WithValue(context.Background(), "user_id", int64(1001))
	ctx = context.WithValue(ctx, "sid", sessionID)
	ctx = context.WithValue(ctx, "session_version", int64(1))
	l := NewMeLogic(ctx, svcCtx)

	resp, err := l.Me()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.UserId != 1001 {
		t.Fatalf("unexpected user_id: %d", resp.UserId)
	}
	if resp.DisplayName == "" {
		t.Fatalf("expected display_name")
	}
}

func TestMeLogic_Me_MissingClaim(t *testing.T) {
	l := NewMeLogic(context.Background(), svc.NewServiceContext(config.Config{
		DemoPassword: "pwd",
	}))

	_, err := l.Me()
	if err == nil {
		t.Fatalf("expected auth error")
	}
}

func TestMeLogic_Me_RejectsRevokedSession(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		DemoPassword: "pwd",
	})
	sessionID, refreshToken, err := svcCtx.Store.CreateSession(1001, 3600)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := svcCtx.Store.Logout(refreshToken); err != nil {
		t.Fatalf("logout session: %v", err)
	}

	ctx := context.WithValue(context.Background(), "user_id", int64(1001))
	ctx = context.WithValue(ctx, "sid", sessionID)
	ctx = context.WithValue(ctx, "session_version", int64(1))

	_, err = NewMeLogic(ctx, svcCtx).Me()
	if err == nil {
		t.Fatalf("expected revoked session error")
	}
}

func TestMeLogic_Me_RejectsStaleSessionVersion(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		DemoPassword: "pwd",
	})
	sessionID, _, err := svcCtx.Store.CreateSession(1001, 3600)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ctx := context.WithValue(context.Background(), "user_id", int64(1001))
	ctx = context.WithValue(ctx, "sid", sessionID)
	ctx = context.WithValue(ctx, "session_version", int64(0))

	_, err = NewMeLogic(ctx, svcCtx).Me()
	if err == nil {
		t.Fatalf("expected stale session version error")
	}
}
