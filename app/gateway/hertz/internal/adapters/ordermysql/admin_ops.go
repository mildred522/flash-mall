package ordermysql

import (
	"database/sql"

	"flash-mall/app/gateway/hertz/internal/application/adminops"
)

type AdminOpsRepository struct{ db *sql.DB }

var _ adminops.Repository = (*AdminOpsRepository)(nil)

func NewAdminOpsRepository(db *sql.DB) *AdminOpsRepository {
	return &AdminOpsRepository{db: db}
}
