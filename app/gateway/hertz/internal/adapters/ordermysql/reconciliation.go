package ordermysql

import (
	"database/sql"

	"flash-mall/app/gateway/hertz/internal/application/reconciliation"
)

type ReconciliationRepository struct{ db *sql.DB }

var _ reconciliation.Repository = (*ReconciliationRepository)(nil)

func NewReconciliationRepository(db *sql.DB) *ReconciliationRepository {
	return &ReconciliationRepository{db: db}
}
