package authmysql

import (
	"context"
	"database/sql"

	"flash-mall/app/gateway/hertz/internal/application/useraddress"
)

type UserAddressRepository struct{ db *sql.DB }

var _ useraddress.Repository = (*UserAddressRepository)(nil)

func NewUserAddressRepository(db *sql.DB) *UserAddressRepository {
	return &UserAddressRepository{db: db}
}

func (r *UserAddressRepository) List(ctx context.Context, userID int64) ([]useraddress.Address, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, receiver_name, receiver_phone, province, city, district, detail, is_default
FROM mall_auth.user_address WHERE user_id = ? AND status = 1 ORDER BY is_default DESC, id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]useraddress.Address, 0)
	for rows.Next() {
		var item useraddress.Address
		var isDefault int64
		if err := rows.Scan(&item.AddressID, &item.ReceiverName, &item.ReceiverPhone,
			&item.Province, &item.City, &item.District, &item.Detail, &isDefault); err != nil {
			return nil, err
		}
		item.IsDefault = isDefault == 1
		items = append(items, item)
	}
	return items, rows.Err()
}
