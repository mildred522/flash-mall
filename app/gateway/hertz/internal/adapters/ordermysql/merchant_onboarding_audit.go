package ordermysql

import (
	"context"
	"database/sql"
	"errors"

	"flash-mall/app/gateway/hertz/internal/application/merchantonboarding"
)

func (r *MerchantOnboardingRepository) Audit(
	ctx context.Context,
	input merchantonboarding.AuditInput,
) (merchantonboarding.AuditResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return merchantonboarding.AuditResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var userID, status, merchantID int64
	var name, phone string
	err = tx.QueryRowContext(ctx, `SELECT user_id, merchant_name, contact_phone, status, merchant_id
FROM merchant_apply WHERE id = ? FOR UPDATE`, input.ApplyID).Scan(&userID, &name, &phone, &status, &merchantID)
	if errors.Is(err, sql.ErrNoRows) {
		return merchantonboarding.AuditResult{Rejection: merchantonboarding.RejectNotFound}, nil
	}
	if err != nil {
		return merchantonboarding.AuditResult{}, err
	}
	desiredStatus := merchantonboarding.StatusRejected
	if input.Approve {
		desiredStatus = merchantonboarding.StatusApproved
	}
	if status != merchantonboarding.StatusPending {
		if status != desiredStatus {
			return merchantonboarding.AuditResult{Rejection: merchantonboarding.RejectAlreadyAudited}, nil
		}
		if err := tx.Commit(); err != nil {
			return merchantonboarding.AuditResult{}, err
		}
		return merchantonboarding.AuditResult{
			ApplyID: input.ApplyID, MerchantID: merchantID, Status: status, Idempotent: true,
		}, nil
	}

	if input.Approve {
		result, err := tx.ExecContext(ctx,
			"INSERT INTO merchant (name,owner_user_id,status,contact_phone) VALUES (?, ?, 1, ?)", name, userID, phone)
		if err != nil {
			return merchantonboarding.AuditResult{}, err
		}
		merchantID, err = result.LastInsertId()
		if err != nil {
			return merchantonboarding.AuditResult{}, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO merchant_user (merchant_id,user_id,role,status)
VALUES (?,?,'owner',1) ON DUPLICATE KEY UPDATE role=VALUES(role),status=VALUES(status)`, merchantID, userID); err != nil {
			return merchantonboarding.AuditResult{}, err
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE merchant_apply
SET status=?,merchant_id=?,audit_remark=?,operator_id=?,audit_time=NOW()
WHERE id=? AND status=0`, desiredStatus, merchantID, input.Remark, input.OperatorID, input.ApplyID)
	if err != nil {
		return merchantonboarding.AuditResult{}, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return merchantonboarding.AuditResult{}, err
	}
	if rows != 1 {
		return merchantonboarding.AuditResult{Rejection: merchantonboarding.RejectAlreadyAudited}, nil
	}
	if err := tx.Commit(); err != nil {
		return merchantonboarding.AuditResult{}, err
	}
	return merchantonboarding.AuditResult{
		ApplyID: input.ApplyID, MerchantID: merchantID, Status: desiredStatus,
	}, nil
}
