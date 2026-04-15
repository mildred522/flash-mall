package svc

import (
	"testing"
	"time"

	"flash-mall/app/auth/api/internal/authstore"
	"flash-mall/app/auth/api/internal/config"
)

type stubAuthStore struct{}

func (s *stubAuthStore) IssueCode(string, string, int64) (string, time.Time, error) {
	return "", time.Time{}, nil
}

func (s *stubAuthStore) ConsumeCode(string, string, string) error {
	return nil
}

func (s *stubAuthStore) CreateUser(string, string, string) (*authstore.User, error) {
	return nil, nil
}

func (s *stubAuthStore) Authenticate(int64, string, string) (*authstore.User, error) {
	return nil, nil
}

func (s *stubAuthStore) GetUserByPhone(string) (*authstore.User, bool) {
	return nil, false
}

func (s *stubAuthStore) GetUserByID(int64) (*authstore.User, bool) {
	return nil, false
}

func (s *stubAuthStore) GetActiveSession(string) (*authstore.Session, bool) {
	return nil, false
}

func (s *stubAuthStore) CreateSession(int64, int64) (string, string, error) {
	return "", "", nil
}

func (s *stubAuthStore) CreateSessionForDevice(int64, string, int64) (string, string, error) {
	return "", "", nil
}

func (s *stubAuthStore) RefreshSession(string, int64) (*authstore.Session, string, error) {
	return nil, "", nil
}

func (s *stubAuthStore) Logout(string) error {
	return nil
}

func (s *stubAuthStore) LogoutAll(int64) {}

func (s *stubAuthStore) UpdatePassword(string, string) (*authstore.User, error) {
	return nil, nil
}

func TestNewServiceContextWithStore_UsesInjectedStore(t *testing.T) {
	store := &stubAuthStore{}

	svcCtx := NewServiceContextWithStore(config.Config{
		DemoPassword: "pwd",
	}, store)

	if svcCtx.Store != authstore.AuthStore(store) {
		t.Fatalf("expected injected store to be preserved")
	}
	if svcCtx.Config.RefreshCookieName != "fm_refresh_token" {
		t.Fatalf("unexpected refresh cookie name: %s", svcCtx.Config.RefreshCookieName)
	}
	if svcCtx.Config.CodeTTLSeconds != 300 {
		t.Fatalf("unexpected code ttl: %d", svcCtx.Config.CodeTTLSeconds)
	}
}

func TestNewServiceContext_UsesSQLStoreWhenDataSourceConfigured(t *testing.T) {
	svcCtx := NewServiceContext(config.Config{
		DemoPassword: "pwd",
		DataSource:   "root:pwd@tcp(127.0.0.1:3306)/mall_auth?charset=utf8mb4&parseTime=true&loc=Local",
	})

	if _, ok := svcCtx.Store.(*authstore.SQLStore); !ok {
		t.Fatalf("expected SQLStore when datasource is configured, got %T", svcCtx.Store)
	}
}
