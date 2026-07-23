package ordermysql

import (
	"context"
	"database/sql"
	"errors"

	"flash-mall/app/common/orderstatus"
	"flash-mall/app/gateway/hertz/internal/application/merchantquery"
)

type MerchantProfileRepository struct{ db *sql.DB }

var _ merchantquery.Repository = (*MerchantProfileRepository)(nil)

func NewMerchantProfileRepository(db *sql.DB) *MerchantProfileRepository {
	return &MerchantProfileRepository{db: db}
}

func (r *MerchantProfileRepository) MerchantsByUser(ctx context.Context, userID int64) ([]merchantquery.Merchant, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT m.id, m.name, mu.role, m.status
FROM merchant_user mu
JOIN merchant m ON m.id = mu.merchant_id
WHERE mu.user_id = ? AND mu.status = 1
ORDER BY mu.id ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]merchantquery.Merchant, 0)
	for rows.Next() {
		var item merchantquery.Merchant
		if err := rows.Scan(&item.MerchantID, &item.Name, &item.Role, &item.Status); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *MerchantProfileRepository) LatestApplication(ctx context.Context, userID int64) (merchantquery.Application, bool, error) {
	var item merchantquery.Application
	err := r.db.QueryRowContext(ctx, `SELECT id, merchant_name, contact_phone, status, merchant_id, audit_remark,
       COALESCE(DATE_FORMAT(create_time, '%Y-%m-%d %H:%i:%s'), ''),
       COALESCE(DATE_FORMAT(audit_time, '%Y-%m-%d %H:%i:%s'), '')
FROM merchant_apply
WHERE user_id = ?
ORDER BY id DESC
LIMIT 1`, userID).Scan(&item.ApplyID, &item.MerchantName, &item.ContactPhone, &item.Status,
		&item.MerchantID, &item.AuditRemark, &item.CreateTime, &item.AuditTime)
	if errors.Is(err, sql.ErrNoRows) {
		return merchantquery.Application{}, false, nil
	}
	return item, err == nil, err
}

func (r *MerchantProfileRepository) Dashboard(ctx context.Context, merchantID int64) (merchantquery.DashboardStats, error) {
	stats := merchantquery.DashboardStats{MerchantID: merchantID}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*),
       COALESCE(SUM(CASE WHEN status IN (1,3,4,5,6) THEN 1 ELSE 0 END), 0),
       COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0)
FROM orders
WHERE merchant_id = ?`, int64(orderstatus.Paid), merchantID).Scan(
		&stats.OrderCount, &stats.PaidOrderCount, &stats.ShipPendingCount,
	); err != nil {
		return merchantquery.DashboardStats{}, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(s.payable_amount_fen), 0)
FROM orders o
JOIN order_price_snapshot s ON s.order_id = o.id
WHERE o.merchant_id = ? AND o.status IN (1,3,4,5,6)`, merchantID).Scan(&stats.SalesAmountFen); err != nil {
		return merchantquery.DashboardStats{}, err
	}
	if err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM refund_order WHERE merchant_id = ? AND status IN (0,1)", merchantID,
	).Scan(&stats.RefundPending); err != nil {
		return merchantquery.DashboardStats{}, err
	}
	return stats, nil
}

func (r *MerchantProfileRepository) FirstMerchantByUser(ctx context.Context, userID int64) (int64, bool, error) {
	var merchantID int64
	err := r.db.QueryRowContext(ctx, `SELECT merchant_id
FROM merchant_user
WHERE user_id = ? AND status = 1
ORDER BY id ASC
LIMIT 1`, userID).Scan(&merchantID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return merchantID, err == nil, err
}

func (r *MerchantProfileRepository) UserCanAccessMerchant(ctx context.Context, userID, merchantID int64) (bool, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*)
FROM merchant_user
WHERE user_id = ? AND merchant_id = ? AND status = 1`, userID, merchantID).Scan(&count)
	return count > 0, err
}

func (r *MerchantProfileRepository) DefaultActiveMerchant(ctx context.Context) (int64, bool, error) {
	var merchantID int64
	err := r.db.QueryRowContext(ctx, `SELECT id
FROM merchant
WHERE status = 1
ORDER BY id ASC
LIMIT 1`).Scan(&merchantID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return merchantID, err == nil, err
}
