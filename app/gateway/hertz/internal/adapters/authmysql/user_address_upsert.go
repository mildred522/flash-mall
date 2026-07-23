package authmysql

import (
	"context"

	"flash-mall/app/gateway/hertz/internal/application/useraddress"
)

func (r *UserAddressRepository) Upsert(
	ctx context.Context,
	input useraddress.UpsertInput,
) (useraddress.UpsertRecord, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return useraddress.UpsertRecord{}, err
	}
	defer func() { _ = tx.Rollback() }()
	isDefault := int64(0)
	if input.IsDefault {
		isDefault = 1
		if _, err := tx.ExecContext(ctx,
			"UPDATE mall_auth.user_address SET is_default = 0 WHERE user_id = ?", input.UserID); err != nil {
			return useraddress.UpsertRecord{}, err
		}
	}
	if input.AddressID > 0 {
		result, err := tx.ExecContext(ctx, `UPDATE mall_auth.user_address
SET receiver_name=?, receiver_phone=?, province=?, city=?, district=?, detail=?, is_default=?
WHERE id=? AND user_id=? AND status=1`, input.ReceiverName, input.ReceiverPhone,
			input.Province, input.City, input.District, input.Detail, isDefault, input.AddressID, input.UserID)
		if err != nil {
			return useraddress.UpsertRecord{}, err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return useraddress.UpsertRecord{}, err
		}
		if rows == 0 {
			return useraddress.UpsertRecord{Found: false}, nil
		}
		if err := tx.Commit(); err != nil {
			return useraddress.UpsertRecord{}, err
		}
		return useraddress.UpsertRecord{AddressID: input.AddressID, Found: true}, nil
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO mall_auth.user_address
  (user_id, receiver_name, receiver_phone, province, city, district, detail, is_default)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, input.UserID, input.ReceiverName, input.ReceiverPhone,
		input.Province, input.City, input.District, input.Detail, isDefault)
	if err != nil {
		return useraddress.UpsertRecord{}, err
	}
	addressID, err := result.LastInsertId()
	if err != nil {
		return useraddress.UpsertRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return useraddress.UpsertRecord{}, err
	}
	return useraddress.UpsertRecord{AddressID: addressID, Found: true}, nil
}
