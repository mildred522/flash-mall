package ordermysql

import (
	"database/sql"

	"flash-mall/app/gateway/hertz/internal/application/merchantonboarding"
)

type MerchantOnboardingRepository struct{ db *sql.DB }

var _ merchantonboarding.Repository = (*MerchantOnboardingRepository)(nil)

func NewMerchantOnboardingRepository(db *sql.DB) *MerchantOnboardingRepository {
	return &MerchantOnboardingRepository{db: db}
}
