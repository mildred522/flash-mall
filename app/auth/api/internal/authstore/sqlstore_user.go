package authstore

import (
	"context"
	"database/sql"

	"golang.org/x/crypto/bcrypt"
)

func (s *SQLStore) CreateUser(phone, displayName, password string) (*User, error) {
	if phone == "" || password == "" {
		return nil, ErrInvalidCredentials
	}
	if err := s.ensureDemoUser(); err != nil {
		return nil, err
	}

	db, err := s.rawDB()
	if err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx,
		"INSERT INTO users (display_name, status, session_version) VALUES (?, ?, ?)",
		"", statusActive, int64(1),
	)
	if err != nil {
		return nil, err
	}

	userID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	if displayName == "" {
		displayName = defaultDisplayName(userID)
	}
	if _, err = tx.ExecContext(ctx,
		"UPDATE users SET display_name = ? WHERE id = ?",
		displayName, userID,
	); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx,
		"INSERT INTO user_identities (user_id, identity_type, identity_value, is_verified, verified_at) VALUES (?, 'phone', ?, 1, NOW())",
		userID, phone,
	); err != nil {
		if isDuplicateKey(err) {
			return nil, ErrUserExists
		}
		return nil, err
	}
	if _, err = tx.ExecContext(ctx,
		"INSERT INTO user_credentials (user_id, credential_type, password_hash, hash_algo, password_updated_at) VALUES (?, 'password', ?, 'bcrypt', NOW())",
		userID, string(hash),
	); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}

	user := &User{
		ID:             userID,
		Phone:          phone,
		DisplayName:    displayName,
		PasswordHash:   string(hash),
		SessionVersion: 1,
	}
	s.syncUserVersion(ctx, user.ID, user.SessionVersion)
	return cloneUser(user), nil
}

func (s *SQLStore) Authenticate(userID int64, phone, password string) (*User, error) {
	if err := s.ensureDemoUser(); err != nil {
		return nil, err
	}

	var user *User
	var ok bool
	if phone != "" {
		user, ok = s.GetUserByPhone(phone)
	} else {
		user, ok = s.GetUserByID(userID)
	}
	if !ok || user == nil {
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	return cloneUser(user), nil
}

func (s *SQLStore) GetUserByPhone(phone string) (*User, bool) {
	if phone == "" {
		return nil, false
	}
	if err := s.ensureDemoUser(); err != nil {
		return nil, false
	}

	db, err := s.rawDB()
	if err != nil {
		return nil, false
	}

	user, err := queryUser(
		db.QueryRowContext(
			context.Background(),
			`SELECT u.id, i.identity_value, u.display_name, COALESCE(c.password_hash, ''), u.session_version
			 FROM users u
			 JOIN user_identities i ON i.user_id = u.id AND i.identity_type = 'phone'
			 LEFT JOIN user_credentials c ON c.user_id = u.id AND c.credential_type = 'password'
			 WHERE i.identity_value = ? AND u.status = ?
			 LIMIT 1`,
			phone, statusActive,
		),
	)
	if err != nil {
		return nil, false
	}
	return user, true
}

func (s *SQLStore) GetUserByID(userID int64) (*User, bool) {
	if userID <= 0 {
		return nil, false
	}
	if err := s.ensureDemoUser(); err != nil {
		return nil, false
	}

	db, err := s.rawDB()
	if err != nil {
		return nil, false
	}

	user, err := queryUser(
		db.QueryRowContext(
			context.Background(),
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
		return nil, false
	}
	return user, true
}

func (s *SQLStore) UpdatePassword(phone, newPassword string) (*User, error) {
	if phone == "" || newPassword == "" {
		return nil, ErrInvalidCredentials
	}
	if err := s.ensureDemoUser(); err != nil {
		return nil, err
	}

	db, err := s.rawDB()
	if err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	user, err := queryUser(
		tx.QueryRowContext(
			ctx,
			`SELECT u.id, i.identity_value, u.display_name, COALESCE(c.password_hash, ''), u.session_version
			 FROM users u
			 JOIN user_identities i ON i.user_id = u.id AND i.identity_type = 'phone'
			 LEFT JOIN user_credentials c ON c.user_id = u.id AND c.credential_type = 'password'
			 WHERE i.identity_value = ? AND u.status = ?
			 LIMIT 1`,
			phone, statusActive,
		),
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	nextVersion := user.SessionVersion + 1
	if nextVersion <= 0 {
		nextVersion = 1
	}
	if _, err = tx.ExecContext(ctx,
		"UPDATE users SET session_version = ? WHERE id = ?",
		nextVersion, user.ID,
	); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx,
		"UPDATE user_credentials SET password_hash = ?, hash_algo = 'bcrypt', password_updated_at = NOW() WHERE user_id = ? AND credential_type = 'password'",
		string(hash), user.ID,
	); err != nil {
		return nil, err
	}

	sessionIDs, err := listActiveSessionIDsTx(ctx, tx, user.ID, "")
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx,
		"UPDATE user_sessions SET status = ?, revoked_at = NOW() WHERE user_id = ? AND status = ?",
		statusRevoked, user.ID, statusActive,
	); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}

	for _, sessionID := range sessionIDs {
		s.deleteSessionState(sessionID)
	}
	s.syncUserVersion(ctx, user.ID, nextVersion)
	user.PasswordHash = string(hash)
	user.SessionVersion = nextVersion
	return cloneUser(user), nil
}

type userScanner interface {
	Scan(dest ...any) error
}

func queryUser(row userScanner) (*User, error) {
	var user User
	if err := row.Scan(
		&user.ID,
		&user.Phone,
		&user.DisplayName,
		&user.PasswordHash,
		&user.SessionVersion,
	); err != nil {
		return nil, err
	}
	if user.DisplayName == "" {
		user.DisplayName = defaultDisplayName(user.ID)
	}
	return &user, nil
}
