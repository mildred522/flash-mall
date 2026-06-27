package sessionstate

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v4"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Validator interface {
	Validate(ctx context.Context, authHeader string) error
}

type HTTPValidator struct {
	client  *http.Client
	baseURL string
}

type SessionSnapshot struct {
	SessionID      string `json:"session_id"`
	UserID         int64  `json:"user_id"`
	DeviceType     string `json:"device_type"`
	SessionVersion int64  `json:"session_version"`
}

type SnapshotStore interface {
	LoadSession(ctx context.Context, sessionID string) (*SessionSnapshot, bool, error)
	LoadUserVersion(ctx context.Context, userID int64) (int64, bool, error)
}

type RedisValidator struct {
	store  SnapshotStore
	secret string
}

type RedisSnapshotStore struct {
	rds *redis.Redis
}

func NewHTTPValidator(client *http.Client, baseURL string) *HTTPValidator {
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPValidator{
		client:  client,
		baseURL: strings.TrimRight(baseURL, "/"),
	}
}

func NewRedisValidator(rds *redis.Redis, secret string) *RedisValidator {
	return NewRedisValidatorWithStore(&RedisSnapshotStore{rds: rds}, secret)
}

func NewRedisValidatorWithStore(store SnapshotStore, secret string) *RedisValidator {
	return &RedisValidator{
		store:  store,
		secret: secret,
	}
}

func (v *HTTPValidator) Validate(ctx context.Context, authHeader string) error {
	if strings.TrimSpace(authHeader) == "" {
		return status.Error(codes.Unauthenticated, "missing authorization header")
	}
	if v.baseURL == "" {
		return status.Error(codes.Unavailable, "auth service not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.baseURL+"/api/auth/me", nil)
	if err != nil {
		return status.Error(codes.Internal, "build session validation request failed")
	}
	req.Header.Set("Authorization", authHeader)

	resp, err := v.client.Do(req)
	if err != nil {
		return status.Error(codes.Unavailable, "auth session validation unavailable")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return status.Error(codes.Unauthenticated, "session invalid or expired")
	}
	if resp.StatusCode == http.StatusNotFound {
		return status.Error(codes.Unauthenticated, "session user not found")
	}
	return status.Error(codes.Unavailable, "auth session validation failed")
}

func (v *RedisValidator) Validate(ctx context.Context, authHeader string) error {
	if strings.TrimSpace(authHeader) == "" {
		return status.Error(codes.Unauthenticated, "missing authorization header")
	}
	if v == nil || v.store == nil || v.secret == "" {
		return status.Error(codes.Unavailable, "redis session validator not configured")
	}

	rawToken := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if rawToken == "" {
		return status.Error(codes.Unauthenticated, "missing bearer token")
	}

	token, err := jwt.Parse(rawToken, func(token *jwt.Token) (any, error) {
		return []byte(v.secret), nil
	})
	if err != nil || !token.Valid {
		return status.Error(codes.Unauthenticated, "invalid access token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return status.Error(codes.Unauthenticated, "invalid access token claims")
	}

	userID, ok := parseInt64Claim(claims["sub"])
	if !ok {
		userID, ok = parseInt64Claim(claims["user_id"])
	}
	if !ok {
		return status.Error(codes.Unauthenticated, "missing user subject in jwt")
	}
	sessionID, _ := claims["sid"].(string)
	if sessionID == "" {
		return status.Error(codes.Unauthenticated, "missing session id in jwt")
	}
	sessionVersion, ok := parseInt64Claim(claims["session_version"])
	if !ok {
		return status.Error(codes.Unauthenticated, "missing session version in jwt")
	}

	snapshot, exists, err := v.store.LoadSession(ctx, sessionID)
	if err != nil {
		return status.Error(codes.Unavailable, "load session snapshot failed")
	}
	if !exists || snapshot == nil {
		return status.Error(codes.Unauthenticated, "session invalid or expired")
	}
	if snapshot.UserID != userID || snapshot.SessionVersion != sessionVersion {
		return status.Error(codes.Unauthenticated, "session snapshot mismatch")
	}

	currentVersion, exists, err := v.store.LoadUserVersion(ctx, userID)
	if err != nil {
		return status.Error(codes.Unavailable, "load user session version failed")
	}
	if !exists || currentVersion != sessionVersion {
		return status.Error(codes.Unauthenticated, "session version mismatch")
	}
	return nil
}

func (s *RedisSnapshotStore) LoadSession(ctx context.Context, sessionID string) (*SessionSnapshot, bool, error) {
	if s == nil || s.rds == nil || sessionID == "" {
		return nil, false, nil
	}
	raw, err := s.rds.GetCtx(ctx, "auth:session:sid:"+sessionID)
	if err != nil {
		return nil, false, err
	}
	if raw == "" {
		return nil, false, nil
	}
	var snapshot SessionSnapshot
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		return nil, false, err
	}
	return &snapshot, true, nil
}

func (s *RedisSnapshotStore) LoadUserVersion(ctx context.Context, userID int64) (int64, bool, error) {
	if s == nil || s.rds == nil || userID <= 0 {
		return 0, false, nil
	}
	raw, err := s.rds.GetCtx(ctx, "auth:session:userver:"+strconv.FormatInt(userID, 10))
	if err != nil {
		return 0, false, err
	}
	if raw == "" {
		return 0, false, nil
	}
	version, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false, err
	}
	return version, true, nil
}

func parseInt64Claim(v any) (int64, bool) {
	switch val := v.(type) {
	case int64:
		return val, true
	case int:
		return int64(val), true
	case float64:
		return int64(val), true
	case json.Number:
		out, err := val.Int64()
		if err != nil {
			return 0, false
		}
		return out, true
	case string:
		out, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return 0, false
		}
		return out, true
	default:
		return 0, false
	}
}
