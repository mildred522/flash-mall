package ordermysql

import (
	"context"
	"database/sql"
	"errors"

	"flash-mall/app/gateway/hertz/internal/application/merchantstore"
)

func (r *MerchantStoreRepository) Update(
	ctx context.Context,
	input merchantstore.UpdateInput,
) (merchantstore.UpdateRecord, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return merchantstore.UpdateRecord{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var merchantStatus int64
	err = tx.QueryRowContext(ctx,
		"SELECT status FROM mall_order.merchant WHERE id = ? FOR UPDATE", input.MerchantID).Scan(&merchantStatus)
	if errors.Is(err, sql.ErrNoRows) || err == nil && merchantStatus != 1 {
		return merchantstore.UpdateRecord{Found: false}, nil
	}
	if err != nil {
		return merchantstore.UpdateRecord{}, err
	}
	var currentVersion int64
	err = tx.QueryRowContext(ctx, `SELECT version FROM mall_order.merchant_store_profile
WHERE merchant_id = ? FOR UPDATE`, input.MerchantID).Scan(&currentVersion)
	if errors.Is(err, sql.ErrNoRows) {
		if input.ExpectedVersion != 0 {
			return merchantstore.UpdateRecord{Found: true, Rejection: merchantstore.RejectVersionConflict}, nil
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO mall_order.merchant_store_profile
  (merchant_id, logo_url, banner_url, description, version) VALUES (?, ?, ?, ?, 1)`,
			input.MerchantID, input.LogoURL, input.BannerURL, input.Description); err != nil {
			return merchantstore.UpdateRecord{}, err
		}
		if err := tx.Commit(); err != nil {
			return merchantstore.UpdateRecord{}, err
		}
		return merchantstore.UpdateRecord{Found: true, Version: 1}, nil
	}
	if err != nil {
		return merchantstore.UpdateRecord{}, err
	}
	if currentVersion != input.ExpectedVersion {
		return merchantstore.UpdateRecord{Found: true, Rejection: merchantstore.RejectVersionConflict}, nil
	}
	result, err := tx.ExecContext(ctx, `UPDATE mall_order.merchant_store_profile
SET logo_url = ?, banner_url = ?, description = ?, version = version + 1
WHERE merchant_id = ? AND version = ?`, input.LogoURL, input.BannerURL, input.Description,
		input.MerchantID, currentVersion)
	if err != nil {
		return merchantstore.UpdateRecord{}, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return merchantstore.UpdateRecord{}, err
	}
	if rows != 1 {
		return merchantstore.UpdateRecord{Found: true, Rejection: merchantstore.RejectVersionConflict}, nil
	}
	if err := tx.Commit(); err != nil {
		return merchantstore.UpdateRecord{}, err
	}
	return merchantstore.UpdateRecord{Found: true, Version: currentVersion + 1}, nil
}
