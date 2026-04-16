package authstore

import (
	"context"
	"database/sql"
	"time"
)

func (s *SQLStore) GetActiveSession(sessionID string) (*Session, bool) {
	if sessionID == "" {
		return nil, false
	}
	if err := s.ensureDemoUser(); err != nil {
		return nil, false
	}

	db, err := s.rawDB()
	if err != nil {
		return nil, false
	}

	session, err := querySession(
		db.QueryRowContext(
			context.Background(),
			`SELECT id, user_id, device_type, session_version, refresh_token_hash, previous_refresh_token_hash, refresh_family_secret, refresh_generation, expires_at, status
			 FROM user_sessions
			 WHERE id = ?
			 LIMIT 1`,
			sessionID,
		),
	)
	if err != nil || session == nil {
		return nil, false
	}
	if session.Revoked || time.Now().After(session.ExpiresAt) {
		return nil, false
	}
	return session, true
}

func (s *SQLStore) CreateSession(userID int64, ttlSeconds int64) (string, string, error) {
	return s.CreateSessionForDevice(userID, "web", ttlSeconds)
}

func (s *SQLStore) CreateSessionForDevice(userID int64, deviceType string, ttlSeconds int64) (string, string, error) {
	if userID <= 0 {
		return "", "", ErrUserNotFound
	}
	if err := s.ensureDemoUser(); err != nil {
		return "", "", err
	}
	if ttlSeconds <= 0 {
		ttlSeconds = 7 * 24 * 60 * 60
	}
	if deviceType == "" {
		deviceType = "web"
	}

	db, err := s.rawDB()
	if err != nil {
		return "", "", err
	}

	sessionID, err := randomToken()
	if err != nil {
		return "", "", err
	}
	refreshFamilySecret, err := randomSecret()
	if err != nil {
		return "", "", err
	}
	refreshToken := buildRefreshToken(sessionID, 1, refreshFamilySecret)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback()

	user, err := queryUser(
		tx.QueryRowContext(
			ctx,
			`SELECT u.id, COALESCE(i.identity_value, ''), u.display_name, COALESCE(c.password_hash, ''), u.session_version
			 FROM users u
			 LEFT JOIN user_identities i ON i.user_id = u.id AND i.identity_type = 'phone'
			 LEFT JOIN user_credentials c ON c.user_id = u.id AND c.credential_type = 'password'
			 WHERE u.id = ? AND u.status = ?
			 LIMIT 1`,
			userID, statusActive,
		),
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", ErrUserNotFound
		}
		return "", "", err
	}

	oldSessionIDs, err := listActiveSessionIDsTx(ctx, tx, userID, deviceType)
	if err != nil {
		return "", "", err
	}
	if len(oldSessionIDs) > 0 {
		if _, err = tx.ExecContext(
			ctx,
			"UPDATE user_sessions SET status = ?, revoked_at = NOW() WHERE user_id = ? AND device_type = ? AND status = ?",
			statusRevoked, userID, deviceType, statusActive,
		); err != nil {
			return "", "", err
		}
	}

	expiresAt := time.Now().Add(time.Duration(ttlSeconds) * time.Second)
	if _, err = tx.ExecContext(
		ctx,
		`INSERT INTO user_sessions (id, user_id, device_type, session_version, refresh_token_hash, previous_refresh_token_hash, refresh_family_secret, refresh_generation, status, expires_at, last_seen_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())`,
		sessionID,
		userID,
		deviceType,
		user.SessionVersion,
		hashToken(refreshToken),
		"",
		refreshFamilySecret,
		int64(1),
		statusActive,
		expiresAt,
	); err != nil {
		return "", "", err
	}
	if err = tx.Commit(); err != nil {
		return "", "", err
	}

	for _, oldSessionID := range oldSessionIDs {
		s.deleteSessionState(oldSessionID)
	}
	s.syncSessionState(&Session{
		ID:                  sessionID,
		UserID:              userID,
		DeviceType:          deviceType,
		SessionVersion:      user.SessionVersion,
		RefreshFamilySecret: refreshFamilySecret,
		RefreshGeneration:   1,
		ExpiresAt:           expiresAt,
	})
	return sessionID, refreshToken, nil
}

func (s *SQLStore) RefreshSession(refreshToken string, ttlSeconds int64) (*Session, string, error) {
	if refreshToken == "" {
		return nil, "", ErrRefreshTokenInvalid
	}
	if err := s.ensureDemoUser(); err != nil {
		return nil, "", err
	}
	if ttlSeconds <= 0 {
		ttlSeconds = 7 * 24 * 60 * 60
	}
	sessionID, tokenGeneration, signature, ok := parseRefreshToken(refreshToken)
	if !ok {
		return nil, "", ErrRefreshTokenInvalid
	}

	db, err := s.rawDB()
	if err != nil {
		return nil, "", err
	}

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback()

	refreshHash := hashToken(refreshToken)
	session, err := querySession(
		tx.QueryRowContext(
			ctx,
			`SELECT id, user_id, device_type, session_version, refresh_token_hash, previous_refresh_token_hash, refresh_family_secret, refresh_generation, expires_at, status
			 FROM user_sessions
			 WHERE id = ?
			 LIMIT 1 FOR UPDATE`,
			sessionID,
		),
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, "", ErrRefreshTokenInvalid
		}
		return nil, "", err
	}
	if session.Revoked || time.Now().After(session.ExpiresAt) {
		return nil, "", ErrRefreshTokenInvalid
	}
	if !verifyRefreshTokenSignature(sessionID, tokenGeneration, signature, session.RefreshFamilySecret) {
		return nil, "", ErrRefreshTokenInvalid
	}
	if tokenGeneration < session.RefreshGeneration {
		if _, err = tx.ExecContext(
			ctx,
			"UPDATE user_sessions SET status = ?, revoked_at = NOW(), last_seen_at = NOW() WHERE id = ? AND status = ?",
			statusRevoked, session.ID, statusActive,
		); err != nil {
			return nil, "", err
		}
		if err = tx.Commit(); err != nil {
			return nil, "", err
		}
		s.deleteSessionState(session.ID)
		return nil, "", ErrRefreshTokenReplayed
	}
	if tokenGeneration > session.RefreshGeneration {
		return nil, "", ErrRefreshTokenInvalid
	}

	newGeneration := session.RefreshGeneration + 1
	newRefreshToken := buildRefreshToken(session.ID, newGeneration, session.RefreshFamilySecret)
	newRefreshHash := hashToken(newRefreshToken)
	session.PreviousRefreshTokenHash = session.RefreshTokenHash
	session.RefreshTokenHash = newRefreshHash
	session.RefreshGeneration = newGeneration
	session.ExpiresAt = time.Now().Add(time.Duration(ttlSeconds) * time.Second)

	result, err := tx.ExecContext(
		ctx,
		`UPDATE user_sessions
		 SET refresh_generation = ?, refresh_token_hash = ?, previous_refresh_token_hash = ?, expires_at = ?, last_seen_at = NOW()
		 WHERE id = ? AND refresh_generation = ? AND refresh_token_hash = ? AND status = ?`,
		session.RefreshGeneration, session.RefreshTokenHash, session.PreviousRefreshTokenHash, session.ExpiresAt, session.ID, tokenGeneration, refreshHash, statusActive,
	)
	if err != nil {
		return nil, "", err
	}
	if rows, err := result.RowsAffected(); err != nil {
		return nil, "", err
	} else if rows == 0 {
		return nil, "", ErrRefreshTokenInvalid
	}
	if err = tx.Commit(); err != nil {
		return nil, "", err
	}

	s.syncSessionState(session)
	return session, newRefreshToken, nil
}

func (s *SQLStore) Logout(refreshToken string) error {
	if refreshToken == "" {
		return ErrRefreshTokenInvalid
	}
	if err := s.ensureDemoUser(); err != nil {
		return err
	}
	sessionID, tokenGeneration, signature, ok := parseRefreshToken(refreshToken)
	if !ok {
		return ErrRefreshTokenInvalid
	}

	db, err := s.rawDB()
	if err != nil {
		return err
	}

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	session, err := querySession(
		tx.QueryRowContext(
			ctx,
			`SELECT id, user_id, device_type, session_version, refresh_token_hash, previous_refresh_token_hash, refresh_family_secret, refresh_generation, expires_at, status
			 FROM user_sessions
			 WHERE id = ?
			 LIMIT 1 FOR UPDATE`,
			sessionID,
		),
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrRefreshTokenInvalid
		}
		return err
	}
	if session.Revoked || time.Now().After(session.ExpiresAt) {
		return ErrRefreshTokenInvalid
	}
	if !verifyRefreshTokenSignature(sessionID, tokenGeneration, signature, session.RefreshFamilySecret) {
		return ErrRefreshTokenInvalid
	}
	if tokenGeneration < session.RefreshGeneration {
		if _, err = tx.ExecContext(
			ctx,
			"UPDATE user_sessions SET status = ?, revoked_at = NOW() WHERE id = ? AND status = ?",
			statusRevoked, session.ID, statusActive,
		); err != nil {
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
		s.deleteSessionState(session.ID)
		return ErrRefreshTokenReplayed
	}
	if tokenGeneration > session.RefreshGeneration {
		return ErrRefreshTokenInvalid
	}
	if _, err = tx.ExecContext(
		ctx,
		"UPDATE user_sessions SET status = ?, revoked_at = NOW() WHERE id = ? AND status = ?",
		statusRevoked, session.ID, statusActive,
	); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	s.deleteSessionState(session.ID)
	return nil
}

func (s *SQLStore) LogoutAll(userID int64) {
	if userID <= 0 || s == nil {
		return
	}
	if err := s.ensureDemoUser(); err != nil {
		return
	}

	db, err := s.rawDB()
	if err != nil {
		return
	}

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()

	sessionIDs, err := listActiveSessionIDsTx(ctx, tx, userID, "")
	if err != nil {
		return
	}

	var nextVersion int64
	err = tx.QueryRowContext(
		ctx,
		"SELECT session_version FROM users WHERE id = ? LIMIT 1",
		userID,
	).Scan(&nextVersion)
	if err != nil {
		return
	}
	nextVersion++
	if nextVersion <= 0 {
		nextVersion = 1
	}

	if _, err = tx.ExecContext(
		ctx,
		"UPDATE users SET session_version = ? WHERE id = ?",
		nextVersion, userID,
	); err != nil {
		return
	}
	if _, err = tx.ExecContext(
		ctx,
		"UPDATE user_sessions SET status = ?, revoked_at = NOW() WHERE user_id = ? AND status = ?",
		statusRevoked, userID, statusActive,
	); err != nil {
		return
	}
	if err = tx.Commit(); err != nil {
		return
	}

	for _, sessionID := range sessionIDs {
		s.deleteSessionState(sessionID)
	}
	s.syncUserVersion(ctx, userID, nextVersion)
}

type sessionScanner interface {
	Scan(dest ...any) error
}

func querySession(row sessionScanner) (*Session, error) {
	var (
		session Session
		status  int
	)
	if err := row.Scan(
		&session.ID,
		&session.UserID,
		&session.DeviceType,
		&session.SessionVersion,
		&session.RefreshTokenHash,
		&session.PreviousRefreshTokenHash,
		&session.RefreshFamilySecret,
		&session.RefreshGeneration,
		&session.ExpiresAt,
		&status,
	); err != nil {
		return nil, err
	}
	session.Revoked = status != statusActive
	return cloneSession(&session), nil
}

func listActiveSessionIDsTx(ctx context.Context, tx *sql.Tx, userID int64, deviceType string) ([]string, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if deviceType == "" {
		rows, err = tx.QueryContext(
			ctx,
			"SELECT id FROM user_sessions WHERE user_id = ? AND status = ?",
			userID, statusActive,
		)
	} else {
		rows, err = tx.QueryContext(
			ctx,
			"SELECT id FROM user_sessions WHERE user_id = ? AND device_type = ? AND status = ?",
			userID, deviceType, statusActive,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessionIDs := make([]string, 0)
	for rows.Next() {
		var sessionID string
		if err := rows.Scan(&sessionID); err != nil {
			return nil, err
		}
		sessionIDs = append(sessionIDs, sessionID)
	}
	return sessionIDs, rows.Err()
}
