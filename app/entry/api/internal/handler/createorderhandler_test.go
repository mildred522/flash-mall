//nolint:staticcheck // Tests intentionally mimic go-zero JWT claim string keys.
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"flash-mall/app/entry/api/internal/svc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type stubSessionValidator struct {
	called bool
	err    error
}

func (s *stubSessionValidator) Validate(_ context.Context, _ string) error {
	s.called = true
	return s.err
}

func TestParseUserIDClaim(t *testing.T) {
	cases := []struct {
		name string
		in   any
		ok   bool
		val  int64
	}{
		{name: "int64", in: int64(10), ok: true, val: 10},
		{name: "float64", in: float64(11), ok: true, val: 11},
		{name: "json number", in: json.Number("12"), ok: true, val: 12},
		{name: "string", in: "13", ok: true, val: 13},
		{name: "bad string", in: "abc", ok: false},
		{name: "nil", in: nil, ok: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseUserIDClaim(tc.in)
			if ok != tc.ok {
				t.Fatalf("ok mismatch: got=%v want=%v", ok, tc.ok)
			}
			if ok && got != tc.val {
				t.Fatalf("value mismatch: got=%d want=%d", got, tc.val)
			}
		})
	}
}

func TestExtractAuthIdentity(t *testing.T) {
	t.Run("prefer user_id claim", func(t *testing.T) {
		ctx := context.Background()
		ctx = context.WithValue(ctx, "user_id", int64(101))
		ctx = context.WithValue(ctx, "sub", "202")
		ctx = context.WithValue(ctx, "sid", "session-a")
		ctx = context.WithValue(ctx, "session_version", int64(3))

		identity, err := extractAuthIdentity(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if identity.UserID != 101 {
			t.Fatalf("unexpected user_id: %d", identity.UserID)
		}
		if identity.SessionID != "session-a" {
			t.Fatalf("unexpected session_id: %s", identity.SessionID)
		}
		if identity.SessionVersion != 3 {
			t.Fatalf("unexpected session_version: %d", identity.SessionVersion)
		}
	})

	t.Run("fallback to sub claim", func(t *testing.T) {
		ctx := context.Background()
		ctx = context.WithValue(ctx, "sub", "303")
		ctx = context.WithValue(ctx, "sid", "session-b")
		ctx = context.WithValue(ctx, "session_version", "5")

		identity, err := extractAuthIdentity(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if identity.UserID != 303 {
			t.Fatalf("unexpected user_id: %d", identity.UserID)
		}
		if identity.SessionID != "session-b" {
			t.Fatalf("unexpected session_id: %s", identity.SessionID)
		}
		if identity.SessionVersion != 5 {
			t.Fatalf("unexpected session_version: %d", identity.SessionVersion)
		}
	})

	t.Run("missing subject", func(t *testing.T) {
		_, err := extractAuthIdentity(context.Background())
		if err == nil {
			t.Fatalf("expected auth identity error")
		}
	})

	t.Run("missing session id", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "sub", "404")
		ctx = context.WithValue(ctx, "session_version", int64(1))

		_, err := extractAuthIdentity(ctx)
		if err == nil {
			t.Fatalf("expected session id error")
		}
	})

	t.Run("missing session version", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "sub", "405")
		ctx = context.WithValue(ctx, "sid", "session-c")

		_, err := extractAuthIdentity(ctx)
		if err == nil {
			t.Fatalf("expected session version error")
		}
	})
}

func TestCreateOrderHandler_UsesSessionValidator(t *testing.T) {
	validator := &stubSessionValidator{
		err: status.Error(codes.Unauthenticated, "session invalid or expired"),
	}
	svcCtx := &svc.ServiceContext{
		SessionValidator: validator,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/order/create", bytes.NewBufferString(`{"request_id":"r-1","user_id":1001,"product_id":1,"amount":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer session-token")
	rec := httptest.NewRecorder()

	CreateOrderHandler(svcCtx).ServeHTTP(rec, req)

	if !validator.called {
		t.Fatalf("expected session validator to be called")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
}
