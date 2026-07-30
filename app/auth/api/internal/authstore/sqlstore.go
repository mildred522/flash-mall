package authstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"flash-mall/app/auth/api/internal/sessionstate"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	statusActive  = 1
	statusUsed    = 2
	statusRevoked = 3

	userStatusDisabled = 2
)

type SQLStore struct {
	conn       sqlx.SqlConn
	stateStore sessionstate.StateStore
}

func NewSQLStore(conn sqlx.SqlConn, stateStore sessionstate.StateStore) *SQLStore {
	return &SQLStore{
		conn:       conn,
		stateStore: stateStore,
	}
}

func (s *SQLStore) rawDB() (*sql.DB, error) {
	if s == nil || s.conn == nil {
		return nil, errors.New("sql conn not configured")
	}
	return s.conn.RawDB()
}

func (s *SQLStore) syncSessionState(session *Session) {
	if s == nil || s.stateStore == nil || session == nil {
		return
	}
	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		return
	}
	_ = s.stateStore.SaveSession(context.Background(), sessionstate.SessionSnapshot{
		SessionID:      session.ID,
		UserID:         session.UserID,
		DeviceType:     session.DeviceType,
		SessionVersion: session.SessionVersion,
	}, ttl)
}

func (s *SQLStore) deleteSessionState(sessionID string) {
	if s == nil || s.stateStore == nil || sessionID == "" {
		return
	}
	_ = s.stateStore.DeleteSession(context.Background(), sessionID)
}

func (s *SQLStore) syncUserVersion(ctx context.Context, userID, version int64) {
	if s == nil || s.stateStore == nil || userID <= 0 || version <= 0 {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	_ = s.stateStore.SetUserVersion(ctx, userID, version)
}

func cloneUser(user *User) *User {
	if user == nil {
		return nil
	}
	copy := *user
	return &copy
}

func defaultDisplayName(userID int64) string {
	if userID > 0 {
		return fmt.Sprintf("Flash Mall User %d", userID)
	}
	return "Flash Mall User"
}

func isDuplicateKey(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
