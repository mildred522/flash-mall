package sessionstate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v4"
)

func TestHTTPValidatorValidate(t *testing.T) {
	t.Run("accepts active session", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/auth/me" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
				t.Fatalf("unexpected authorization header: %s", got)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer upstream.Close()

		validator := NewHTTPValidator(upstream.Client(), upstream.URL)
		if err := validator.Validate(context.Background(), "Bearer test-token"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects invalid session", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer upstream.Close()

		validator := NewHTTPValidator(upstream.Client(), upstream.URL)
		if err := validator.Validate(context.Background(), "Bearer revoked-token"); err == nil {
			t.Fatalf("expected invalid session error")
		}
	})

	t.Run("requires authorization header", func(t *testing.T) {
		validator := NewHTTPValidator(http.DefaultClient, "http://127.0.0.1:8890")
		if err := validator.Validate(context.Background(), ""); err == nil {
			t.Fatalf("expected authorization error")
		}
	})
}

type stubSnapshotStore struct {
	session SessionSnapshot
	version int64
	ok      bool
}

func (s *stubSnapshotStore) LoadSession(_ context.Context, _ string) (*SessionSnapshot, bool, error) {
	if !s.ok {
		return nil, false, nil
	}
	copy := s.session
	return &copy, true, nil
}

func (s *stubSnapshotStore) LoadUserVersion(_ context.Context, _ int64) (int64, bool, error) {
	if !s.ok {
		return 0, false, nil
	}
	return s.version, true, nil
}

func TestRedisValidatorValidate(t *testing.T) {
	makeToken := func(t *testing.T, claims jwt.MapClaims) string {
		t.Helper()
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		raw, err := token.SignedString([]byte("test-order-jwt-secret"))
		if err != nil {
			t.Fatalf("sign token: %v", err)
		}
		return raw
	}

	t.Run("accepts active session snapshot", func(t *testing.T) {
		store := &stubSnapshotStore{
			ok: true,
			session: SessionSnapshot{
				SessionID:      "sid-1",
				UserID:         1001,
				SessionVersion: 2,
			},
			version: 2,
		}
		validator := NewRedisValidatorWithStore(store, "test-order-jwt-secret")
		token := makeToken(t, jwt.MapClaims{
			"sub":             "1001",
			"sid":             "sid-1",
			"session_version": 2,
		})
		if err := validator.Validate(context.Background(), "Bearer "+token); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects missing session snapshot", func(t *testing.T) {
		store := &stubSnapshotStore{}
		validator := NewRedisValidatorWithStore(store, "test-order-jwt-secret")
		token := makeToken(t, jwt.MapClaims{
			"sub":             "1001",
			"sid":             "sid-missing",
			"session_version": 2,
		})
		if err := validator.Validate(context.Background(), "Bearer "+token); err == nil {
			t.Fatalf("expected invalid session error")
		}
	})

	t.Run("rejects stale user session version", func(t *testing.T) {
		store := &stubSnapshotStore{
			ok: true,
			session: SessionSnapshot{
				SessionID:      "sid-2",
				UserID:         1001,
				SessionVersion: 2,
			},
			version: 3,
		}
		validator := NewRedisValidatorWithStore(store, "test-order-jwt-secret")
		token := makeToken(t, jwt.MapClaims{
			"sub":             "1001",
			"sid":             "sid-2",
			"session_version": 2,
		})
		if err := validator.Validate(context.Background(), "Bearer "+token); err == nil {
			t.Fatalf("expected stale version error")
		}
	})
}
