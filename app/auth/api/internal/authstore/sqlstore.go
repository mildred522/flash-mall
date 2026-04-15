package authstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"flash-mall/app/auth/api/internal/sessionstate"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"golang.org/x/crypto/bcrypt"
)

const (
	statusActive  = 1
	statusUsed    = 2
	statusRevoked = 3
)

type SQLStore struct {
	conn         sqlx.SqlConn
	stateStore   sessionstate.StateStore
	demoPassword string
	demoOnce     sync.Once
	demoErr      error
}

func NewSQLStore(conn sqlx.SqlConn, demoPassword string, stateStore sessionstate.StateStore) *SQLStore {
	if demoPassword == "" {
		demoPassword = "flashmall123"
	}
	return &SQLStore{
		conn:         conn,
		stateStore:   stateStore,
		demoPassword: demoPassword,
	}
}

func (s *SQLStore) rawDB() (*sql.DB, error) {
	if s == nil || s.conn == nil {
		return nil, errors.New("sql conn not configured")
	}
	return s.conn.RawDB()
}

func (s *SQLStore) ensureDemoUser() error {
	if s == nil {
		return errors.New("sql store not configured")
	}

	s.demoOnce.Do(func() {
		db, err := s.rawDB()
		if err != nil {
			s.demoErr = err
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(s.demoPassword), bcrypt.DefaultCost)
		if err != nil {
			s.demoErr = err
			return
		}

		ctx := context.Background()
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			s.demoErr = err
			return
		}
		defer tx.Rollback()

		if _, err = tx.ExecContext(ctx,
			"INSERT IGNORE INTO users (id, display_name, status, session_version) VALUES (?, ?, ?, ?)",
			int64(1001), "Flash Mall User 1001", statusActive, int64(1),
		); err != nil {
			s.demoErr = err
			return
		}
		if _, err = tx.ExecContext(ctx,
			"INSERT IGNORE INTO user_identities (user_id, identity_type, identity_value, is_verified, verified_at) VALUES (?, 'phone', ?, 1, NOW())",
			int64(1001), "13800000001",
		); err != nil {
			s.demoErr = err
			return
		}
		if _, err = tx.ExecContext(ctx,
			"INSERT IGNORE INTO user_credentials (user_id, credential_type, password_hash, hash_algo, password_updated_at) VALUES (?, 'password', ?, 'bcrypt', NOW())",
			int64(1001), string(hash),
		); err != nil {
			s.demoErr = err
			return
		}
		if err = tx.Commit(); err != nil {
			s.demoErr = err
			return
		}
		s.syncUserVersion(context.Background(), 1001, 1)
	})

	return s.demoErr
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
