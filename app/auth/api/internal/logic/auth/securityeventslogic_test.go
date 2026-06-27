//nolint:staticcheck // Tests intentionally mimic go-zero JWT claim string keys.
package auth

import (
	"context"
	"testing"
	"time"

	"flash-mall/app/auth/api/internal/audit"
	"flash-mall/app/auth/api/internal/config"
	"flash-mall/app/auth/api/internal/svc"
)

func TestSecurityEventsLogic_ListRecentNewestFirst(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		JwtAuthSecret: "test-auth-jwt-secret",
	})

	first := audit.Event{
		EventType:     "login_password_fail",
		Result:        "fail",
		UserID:        1001,
		IdentityValue: "13800000001",
		IP:            "203.0.113.5",
		UserAgent:     "browser-a",
		CreatedAt:     time.Unix(100, 0),
	}
	second := audit.Event{
		EventType:     "login_password_success",
		Result:        "success",
		UserID:        1001,
		IdentityValue: "13800000001",
		IP:            "203.0.113.5",
		UserAgent:     "browser-b",
		CreatedAt:     time.Unix(200, 0),
	}

	if err := svcCtx.AuditRecorder.Record(context.Background(), first); err != nil {
		t.Fatalf("record first event: %v", err)
	}
	if err := svcCtx.AuditRecorder.Record(context.Background(), second); err != nil {
		t.Fatalf("record second event: %v", err)
	}

	ctx := context.WithValue(context.Background(), "user_id", int64(1001))
	resp, err := NewSecurityEventsLogic(ctx, svcCtx).ListRecent()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("unexpected item count: %d", len(resp.Items))
	}
	if resp.Items[0].EventType != second.EventType {
		t.Fatalf("expected newest event first, got %#v", resp.Items)
	}
	if resp.Items[1].EventType != first.EventType {
		t.Fatalf("expected older event second, got %#v", resp.Items)
	}
}

func TestSecurityEventsLogic_ListRecent_FiltersToCurrentUser(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		JwtAuthSecret: "test-auth-jwt-secret",
	})

	for _, item := range []audit.Event{
		{
			EventType:     "login_password_success",
			Result:        "success",
			UserID:        1001,
			IdentityValue: "13800000001",
			IP:            "203.0.113.5",
			CreatedAt:     time.Unix(100, 0),
		},
		{
			EventType:     "send_code_success",
			Result:        "success",
			UserID:        1002,
			IdentityValue: "13800000002",
			IP:            "203.0.113.6",
			CreatedAt:     time.Unix(200, 0),
		},
		{
			EventType:     "refresh_success",
			Result:        "success",
			UserID:        1001,
			IdentityValue: "13800000001",
			IP:            "203.0.113.5",
			CreatedAt:     time.Unix(300, 0),
		},
	} {
		if err := svcCtx.AuditRecorder.Record(context.Background(), item); err != nil {
			t.Fatalf("record event: %v", err)
		}
	}

	ctx := context.WithValue(context.Background(), "user_id", int64(1001))
	resp, err := NewSecurityEventsLogic(ctx, svcCtx).ListRecent()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("expected only current-user items, got %#v", resp.Items)
	}
	if resp.Items[0].EventType != "refresh_success" || resp.Items[1].EventType != "login_password_success" {
		t.Fatalf("unexpected ordering/filter result: %#v", resp.Items)
	}
	for _, item := range resp.Items {
		if item.UserId != 1001 {
			t.Fatalf("unexpected foreign event leaked: %#v", item)
		}
	}
}
