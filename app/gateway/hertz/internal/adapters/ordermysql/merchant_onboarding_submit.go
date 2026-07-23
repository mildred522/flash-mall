package ordermysql

import (
	"context"
	"database/sql"
	"errors"

	"flash-mall/app/gateway/hertz/internal/application/merchantonboarding"
)

func (r *MerchantOnboardingRepository) Submit(
	ctx context.Context,
	input merchantonboarding.SubmitInput,
) (merchantonboarding.SubmitResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return merchantonboarding.SubmitResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var latestID, latestStatus int64
	err = tx.QueryRowContext(ctx, `SELECT id, status FROM merchant_apply
WHERE user_id = ? ORDER BY id DESC LIMIT 1 FOR UPDATE`, input.UserID).Scan(&latestID, &latestStatus)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return merchantonboarding.SubmitResult{}, err
	}
	hasLatest := err == nil

	var merchantID int64
	err = tx.QueryRowContext(ctx, `SELECT mu.merchant_id FROM merchant_user mu
JOIN merchant m ON m.id = mu.merchant_id
WHERE mu.user_id = ? AND mu.status = 1 AND m.status = 1
ORDER BY mu.id ASC LIMIT 1 FOR UPDATE`, input.UserID).Scan(&merchantID)
	if err == nil {
		return merchantonboarding.SubmitResult{Rejection: merchantonboarding.RejectAlreadyActive}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return merchantonboarding.SubmitResult{}, err
	}

	if hasLatest && latestStatus == merchantonboarding.StatusPending {
		if err := tx.Commit(); err != nil {
			return merchantonboarding.SubmitResult{}, err
		}
		return merchantonboarding.SubmitResult{ApplyID: latestID, Status: latestStatus}, nil
	}
	result, err := tx.ExecContext(ctx,
		"INSERT INTO merchant_apply (user_id, merchant_name, contact_phone, status) VALUES (?, ?, ?, 0)",
		input.UserID, input.MerchantName, input.ContactPhone)
	if err != nil {
		return merchantonboarding.SubmitResult{}, err
	}
	applyID, err := result.LastInsertId()
	if err != nil {
		return merchantonboarding.SubmitResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return merchantonboarding.SubmitResult{}, err
	}
	return merchantonboarding.SubmitResult{ApplyID: applyID, Status: merchantonboarding.StatusPending}, nil
}
