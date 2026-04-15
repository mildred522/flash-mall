package authstore

import (
	"context"
	"database/sql"
	"time"
)

func (s *SQLStore) IssueCode(phone, scene string, ttlSeconds int64) (string, time.Time, error) {
	if phone == "" || scene == "" {
		return "", time.Time{}, ErrInvalidCode
	}
	if err := s.ensureDemoUser(); err != nil {
		return "", time.Time{}, err
	}
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}

	db, err := s.rawDB()
	if err != nil {
		return "", time.Time{}, err
	}

	code := "246810"
	expiresAt := time.Now().Add(time.Duration(ttlSeconds) * time.Second)
	_, err = db.ExecContext(
		context.Background(),
		`INSERT INTO verification_codes (target, scene, code_hash, status, expires_at, send_count)
		 VALUES (?, ?, ?, ?, ?, 1)`,
		phone,
		scene,
		hashToken(code),
		statusActive,
		expiresAt,
	)
	if err != nil {
		return "", time.Time{}, err
	}
	return code, expiresAt, nil
}

func (s *SQLStore) ConsumeCode(phone, scene, code string) error {
	if phone == "" || scene == "" || code == "" {
		return ErrInvalidCode
	}
	if err := s.ensureDemoUser(); err != nil {
		return err
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

	var (
		id        int64
		codeHash  string
		expiresAt time.Time
	)
	err = tx.QueryRowContext(
		ctx,
		`SELECT id, code_hash, expires_at
		 FROM verification_codes
		 WHERE target = ? AND scene = ? AND status = ?
		 ORDER BY id DESC
		 LIMIT 1`,
		phone, scene, statusActive,
	).Scan(&id, &codeHash, &expiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrInvalidCode
		}
		return err
	}

	if time.Now().After(expiresAt) {
		if _, err = tx.ExecContext(
			ctx,
			"UPDATE verification_codes SET status = ? WHERE id = ?",
			statusRevoked, id,
		); err != nil {
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
		return ErrInvalidCode
	}
	if codeHash != hashToken(code) {
		return ErrInvalidCode
	}

	if _, err = tx.ExecContext(
		ctx,
		"UPDATE verification_codes SET status = ?, consumed_at = NOW() WHERE id = ?",
		statusUsed, id,
	); err != nil {
		return err
	}
	return tx.Commit()
}
