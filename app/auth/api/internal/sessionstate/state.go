package sessionstate

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	sessionKeyPrefix     = "auth:session:sid:"
	userVersionKeyPrefix = "auth:session:userver:"
	redisWriteTimeout    = time.Second
)

type SessionSnapshot struct {
	SessionID      string `json:"session_id"`
	UserID         int64  `json:"user_id"`
	DeviceType     string `json:"device_type"`
	SessionVersion int64  `json:"session_version"`
}

type StateStore interface {
	SaveSession(ctx context.Context, session SessionSnapshot, ttl time.Duration) error
	DeleteSession(ctx context.Context, sessionID string) error
	SetUserVersion(ctx context.Context, userID int64, version int64) error
}

type RedisStateStore struct {
	rds *redis.Redis
}

func NewRedisStateStore(rds *redis.Redis) *RedisStateStore {
	return &RedisStateStore{rds: rds}
}

func (s *RedisStateStore) SaveSession(ctx context.Context, session SessionSnapshot, ttl time.Duration) error {
	if s == nil || s.rds == nil {
		return nil
	}
	ctx, cancel := withRedisTimeout(ctx)
	defer cancel()
	payload, err := json.Marshal(session)
	if err != nil {
		return err
	}
	return s.rds.PipelinedCtx(ctx, func(pipe redis.Pipeliner) error {
		pipe.Set(ctx, sessionKey(session.SessionID), string(payload), ttl)
		pipe.Set(ctx, userVersionKey(session.UserID), fmt.Sprintf("%d", session.SessionVersion), 0)
		return nil
	})
}

func (s *RedisStateStore) DeleteSession(ctx context.Context, sessionID string) error {
	if s == nil || s.rds == nil {
		return nil
	}
	ctx, cancel := withRedisTimeout(ctx)
	defer cancel()
	_, err := s.rds.DelCtx(ctx, sessionKey(sessionID))
	return err
}

func (s *RedisStateStore) SetUserVersion(ctx context.Context, userID int64, version int64) error {
	if s == nil || s.rds == nil {
		return nil
	}
	ctx, cancel := withRedisTimeout(ctx)
	defer cancel()
	return s.rds.SetCtx(ctx, userVersionKey(userID), fmt.Sprintf("%d", version))
}

func sessionKey(sessionID string) string {
	return sessionKeyPrefix + sessionID
}

func userVersionKey(userID int64) string {
	return fmt.Sprintf("%s%d", userVersionKeyPrefix, userID)
}

func withRedisTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.WithTimeout(context.Background(), redisWriteTimeout)
	}
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= redisWriteTimeout {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, redisWriteTimeout)
}
