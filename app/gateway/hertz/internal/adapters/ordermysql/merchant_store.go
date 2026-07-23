package ordermysql

import (
	"context"
	"database/sql"
	"errors"

	"flash-mall/app/gateway/hertz/internal/application/merchantstore"
)

type MerchantStoreRepository struct{ db *sql.DB }

var _ merchantstore.Repository = (*MerchantStoreRepository)(nil)

func NewMerchantStoreRepository(db *sql.DB) *MerchantStoreRepository {
	return &MerchantStoreRepository{db: db}
}

func (r *MerchantStoreRepository) Profile(
	ctx context.Context,
	merchantID int64,
) (merchantstore.Profile, bool, error) {
	var profile merchantstore.Profile
	err := r.db.QueryRowContext(ctx, `SELECT m.id, m.name,
       COALESCE(p.logo_url, ''), COALESCE(p.banner_url, ''),
       COALESCE(p.description, ''), COALESCE(p.version, 0)
FROM mall_order.merchant m
LEFT JOIN mall_order.merchant_store_profile p ON p.merchant_id = m.id
WHERE m.id = ? AND m.status = 1`, merchantID).Scan(
		&profile.MerchantID, &profile.MerchantName, &profile.LogoURL,
		&profile.BannerURL, &profile.Description, &profile.Version,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return merchantstore.Profile{}, false, nil
	}
	return profile, err == nil, err
}
