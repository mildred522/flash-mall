package productmysql

import (
	"context"
	"database/sql"

	"flash-mall/app/gateway/hertz/internal/application/campaign"
)

type CampaignRepository struct{ db *sql.DB }

var _ campaign.Repository = (*CampaignRepository)(nil)

func NewCampaignRepository(db *sql.DB) *CampaignRepository { return &CampaignRepository{db: db} }

func (r *CampaignRepository) List(ctx context.Context) ([]campaign.Item, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT c.id, c.product_id, COALESCE(p.name, ''), c.name,
       c.campaign_stock, c.per_user_limit, COALESCE(c.starts_at, ''), COALESCE(c.ends_at, ''), c.status
FROM mall_product.seckill_campaign c
LEFT JOIN mall_product.product p ON p.id = c.product_id
ORDER BY c.id DESC
LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]campaign.Item, 0)
	for rows.Next() {
		var item campaign.Item
		if err := rows.Scan(&item.CampaignID, &item.ProductID, &item.ProductName, &item.Name,
			&item.CampaignStock, &item.PerUserLimit, &item.StartsAt, &item.EndsAt, &item.Status); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *CampaignRepository) Upsert(ctx context.Context, input campaign.UpsertInput) (int64, error) {
	if input.CampaignID > 0 {
		_, err := r.db.ExecContext(ctx, `UPDATE mall_product.seckill_campaign
SET product_id = ?, name = ?, campaign_stock = ?, per_user_limit = ?,
    starts_at = NULLIF(?, ''), ends_at = NULLIF(?, ''), status = ?
WHERE id = ?`, input.ProductID, input.Name, input.CampaignStock, input.PerUserLimit,
			input.StartsAt, input.EndsAt, input.Status, input.CampaignID)
		return input.CampaignID, err
	}
	result, err := r.db.ExecContext(ctx, `INSERT INTO mall_product.seckill_campaign
    (product_id, name, campaign_stock, per_user_limit, starts_at, ends_at, status)
VALUES (?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?)`, input.ProductID, input.Name,
		input.CampaignStock, input.PerUserLimit, input.StartsAt, input.EndsAt, input.Status)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}
