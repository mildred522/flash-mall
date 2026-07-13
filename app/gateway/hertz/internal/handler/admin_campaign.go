package handler

import (
	"context"
	"database/sql"
	"strings"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/svc"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type adminCampaignReq struct {
	CampaignID, ProductID, CampaignStock, PerUserLimit, Status int64
	Name, StartsAt, EndsAt                                     string
}

func AdminCampaignListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		rows, err := db.QueryContext(ctx, `SELECT c.id,c.product_id,COALESCE(p.name,''),c.name,c.campaign_stock,c.per_user_limit,COALESCE(c.starts_at,''),COALESCE(c.ends_at,''),c.status FROM mall_product.seckill_campaign c LEFT JOIN mall_product.product p ON p.id=c.product_id ORDER BY c.id DESC LIMIT 100`)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		defer rows.Close()
		items := make([]map[string]any, 0)
		for rows.Next() {
			var id, pid, stock, limit, status int64
			var pn, name, starts, ends string
			if err := rows.Scan(&id, &pid, &pn, &name, &stock, &limit, &starts, &ends, &status); err != nil {
				fail(ctx, c, consts.StatusBadGateway, err)
				return
			}
			items = append(items, map[string]any{"campaign_id": id, "product_id": pid, "product_name": pn, "name": name, "campaign_stock": stock, "per_user_limit": limit, "starts_at": starts, "ends_at": ends, "status": status})
		}
		ok(ctx, c, map[string]any{"items": items})
	}
}

func AdminCampaignUpsertHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req adminCampaignReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid campaign request"))
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		if req.ProductID <= 0 || req.Name == "" || req.CampaignStock < 0 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "product_id, name and campaign_stock are required"))
			return
		}
		if req.PerUserLimit <= 0 {
			req.PerUserLimit = 1
		}
		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		id := req.CampaignID
		if id > 0 {
			_, err = db.ExecContext(ctx, `UPDATE mall_product.seckill_campaign SET product_id=?,name=?,campaign_stock=?,per_user_limit=?,starts_at=NULLIF(?,''),ends_at=NULLIF(?,''),status=? WHERE id=?`, req.ProductID, req.Name, req.CampaignStock, req.PerUserLimit, req.StartsAt, req.EndsAt, req.Status, id)
		} else {
			var r sql.Result
			r, err = db.ExecContext(ctx, `INSERT INTO mall_product.seckill_campaign (product_id,name,campaign_stock,per_user_limit,starts_at,ends_at,status) VALUES (?,?,?, ?,NULLIF(?,''),NULLIF(?,''),?)`, req.ProductID, req.Name, req.CampaignStock, req.PerUserLimit, req.StartsAt, req.EndsAt, req.Status)
			if err == nil {
				id, _ = r.LastInsertId()
			}
		}
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		ok(ctx, c, map[string]any{"campaign_id": id})
	}
}
