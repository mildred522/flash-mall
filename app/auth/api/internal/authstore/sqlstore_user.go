package authstore

import (
	"context"
	"database/sql"
	"errors"
	"strings"

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
	defer func() { _ = tx.Rollback() }()

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
		Role:           "user",
		Status:         statusActive,
	}
	s.syncUserVersion(ctx, user.ID, user.SessionVersion)
	return cloneUser(user), nil
}

func (s *SQLStore) Authenticate(userID int64, phone, password string) (*User, error) {
	if err := s.ensureDemoUser(); err != nil {
		return nil, err
	}

	var user *User
	var err error
	if phone != "" {
		user, err = s.GetUserByPhone(phone)
	} else {
		user, err = s.GetUserByID(userID)
	}
	if errors.Is(err, ErrUserNotFound) || user == nil {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	return cloneUser(user), nil
}

func (s *SQLStore) GetUserByPhone(phone string) (*User, error) {
	if phone == "" {
		return nil, ErrUserNotFound
	}
	if err := s.ensureDemoUser(); err != nil {
		return nil, err
	}

	db, err := s.rawDB()
	if err != nil {
		return nil, err
	}

	user, err := queryUser(
		db.QueryRowContext(
			context.Background(),
			`SELECT u.id, i.identity_value, u.display_name, COALESCE(c.password_hash, ''), u.session_version, COALESCE(u.role, 'user')
			 FROM users u
			 JOIN user_identities i ON i.user_id = u.id AND i.identity_type = 'phone'
			 LEFT JOIN user_credentials c ON c.user_id = u.id AND c.credential_type = 'password'
			 WHERE i.identity_value = ? AND u.status = ?
			 LIMIT 1`,
			phone, statusActive,
		),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (s *SQLStore) GetUserByID(userID int64) (*User, error) {
	if userID <= 0 {
		return nil, ErrUserNotFound
	}
	if err := s.ensureDemoUser(); err != nil {
		return nil, err
	}

	db, err := s.rawDB()
	if err != nil {
		return nil, err
	}

	user, err := queryUser(
		db.QueryRowContext(
			context.Background(),
			`SELECT u.id, COALESCE(i.identity_value, ''), u.display_name, COALESCE(c.password_hash, ''), u.session_version, COALESCE(u.role, 'user')
			 FROM users u
			 LEFT JOIN user_identities i ON i.user_id = u.id AND i.identity_type = 'phone'
			 LEFT JOIN user_credentials c ON c.user_id = u.id AND c.credential_type = 'password'
			 WHERE u.id = ? AND u.status = ?
			 LIMIT 1`,
			userID, statusActive,
		),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (s *SQLStore) GetUserByIDAnyStatus(userID int64) (*User, error) {
	if userID <= 0 {
		return nil, ErrUserNotFound
	}
	user, err := s.getUserByIDAnyStatus(context.Background(), userID)
	if err != nil {
		return nil, err
	}
	return user, nil
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
	defer func() { _ = tx.Rollback() }()

	user, err := queryUser(
		tx.QueryRowContext(
			ctx,
			`SELECT u.id, i.identity_value, u.display_name, COALESCE(c.password_hash, ''), u.session_version, COALESCE(u.role, 'user')
			 FROM users u
			 JOIN user_identities i ON i.user_id = u.id AND i.identity_type = 'phone'
			 LEFT JOIN user_credentials c ON c.user_id = u.id AND c.credential_type = 'password'
			 WHERE i.identity_value = ? AND u.status = ?
			 LIMIT 1`,
			phone, statusActive,
		),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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

func (s *SQLStore) ListAllUsers() []*User {
	users, _, _ := s.ListUsers(1, 0, 0, "", "")
	return users
}

func (s *SQLStore) ListUsers(page, pageSize, status int64, role, keyword string) ([]*User, int64, error) {
	if err := s.ensureDemoUser(); err != nil {
		return nil, 0, err
	}
	db, err := s.rawDB()
	if err != nil {
		return nil, 0, err
	}

	ctx := context.Background()
	where := "1=1"
	args := []any{}
	if status > 0 {
		where += " AND u.status = ?"
		args = append(args, status)
	}
	if role = strings.TrimSpace(role); role != "" {
		where += " AND COALESCE(u.role, 'user') = ?"
		args = append(args, role)
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		where += " AND (u.display_name LIKE ? OR i.identity_value LIKE ? OR CAST(u.id AS CHAR) LIKE ?)"
		like := "%" + keyword + "%"
		args = append(args, like, like, like)
	}

	var total int64
	countQuery := `SELECT COUNT(DISTINCT u.id)
		 FROM users u
		 LEFT JOIN user_identities i ON i.user_id = u.id AND i.identity_type = 'phone'
		 WHERE ` + where
	if err := db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT u.id, COALESCE(i.identity_value, ''), u.display_name, COALESCE(c.password_hash, ''), u.session_version, COALESCE(u.role, 'user'), u.status, COALESCE(CAST(u.create_time AS CHAR), '')
		 FROM users u
		 LEFT JOIN user_identities i ON i.user_id = u.id AND i.identity_type = 'phone'
		 LEFT JOIN user_credentials c ON c.user_id = u.id AND c.credential_type = 'password'
		 WHERE ` + where + `
		 ORDER BY u.id DESC`
	queryArgs := append([]any{}, args...)
	if pageSize > 0 {
		if page <= 0 {
			page = 1
		}
		query += " LIMIT ? OFFSET ?"
		queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)
	}
	rows, err := db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, total, err
	}
	defer func() { _ = rows.Close() }()

	users := make([]*User, 0)
	for rows.Next() {
		var user User
		if err := rows.Scan(
			&user.ID,
			&user.Phone,
			&user.DisplayName,
			&user.PasswordHash,
			&user.SessionVersion,
			&user.Role,
			&user.Status,
			&user.CreateTime,
		); err != nil {
			return nil, total, err
		}
		if user.DisplayName == "" {
			user.DisplayName = defaultDisplayName(user.ID)
		}
		users = append(users, &user)
	}
	if err := rows.Err(); err != nil {
		return nil, total, err
	}
	return users, total, nil
}

func (s *SQLStore) SetUserStatus(userID int64, newStatus int64) (*User, error) {
	if newStatus != statusActive && newStatus != userStatusDisabled {
		return nil, ErrInvalidCredentials
	}
	db, err := s.rawDB()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var nextVersion int64
	if err := tx.QueryRowContext(ctx, "SELECT session_version + 1 FROM users WHERE id = ? FOR UPDATE", userID).Scan(&nextVersion); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	result, err := tx.ExecContext(ctx, "UPDATE users SET status = ?, session_version = ? WHERE id = ?", newStatus, nextVersion, userID)
	if err != nil {
		return nil, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		return nil, ErrUserNotFound
	}

	sessionIDs := make([]string, 0)
	if newStatus != statusActive {
		rows, err := tx.QueryContext(ctx, "SELECT id FROM user_sessions WHERE user_id = ? AND status = ?", userID, statusActive)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var sessionID string
			if err := rows.Scan(&sessionID); err != nil {
				_ = rows.Close()
				return nil, err
			}
			sessionIDs = append(sessionIDs, sessionID)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE user_sessions SET status = ?, revoked_at = NOW() WHERE user_id = ? AND status = ?", statusRevoked, userID, statusActive); err != nil {
			return nil, err
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	for _, sessionID := range sessionIDs {
		s.deleteSessionState(sessionID)
	}
	s.syncUserVersion(ctx, userID, nextVersion)
	return s.getUserByIDAnyStatus(ctx, userID)
}

func queryUser(row userScanner) (*User, error) {
	var user User
	if err := row.Scan(
		&user.ID,
		&user.Phone,
		&user.DisplayName,
		&user.PasswordHash,
		&user.SessionVersion,
		&user.Role,
	); err != nil {
		return nil, err
	}
	if user.DisplayName == "" {
		user.DisplayName = defaultDisplayName(user.ID)
	}
	if user.Status == 0 {
		user.Status = statusActive
	}
	return &user, nil
}

func (s *SQLStore) getUserByIDAnyStatus(ctx context.Context, userID int64) (*User, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	db, err := s.rawDB()
	if err != nil {
		return nil, err
	}
	var user User
	err = db.QueryRowContext(ctx,
		`SELECT u.id, COALESCE(i.identity_value, ''), u.display_name, COALESCE(c.password_hash, ''), u.session_version, COALESCE(u.role, 'user'), u.status, COALESCE(CAST(u.create_time AS CHAR), '')
		 FROM users u
		 LEFT JOIN user_identities i ON i.user_id = u.id AND i.identity_type = 'phone'
		 LEFT JOIN user_credentials c ON c.user_id = u.id AND c.credential_type = 'password'
		 WHERE u.id = ?`,
		userID,
	).Scan(&user.ID, &user.Phone, &user.DisplayName, &user.PasswordHash, &user.SessionVersion, &user.Role, &user.Status, &user.CreateTime)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	if user.DisplayName == "" {
		user.DisplayName = defaultDisplayName(user.ID)
	}
	return &user, nil
}
