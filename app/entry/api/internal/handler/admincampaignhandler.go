package handler

import (
	"fmt"
	"net/http"
	"strings"

	"flash-mall/app/entry/api/internal/svc"
	"flash-mall/app/entry/api/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func AdminCampaignListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminCampaignListReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if req.Page <= 0 {
			req.Page = 1
		}
		if req.PageSize <= 0 || req.PageSize > 100 {
			req.PageSize = 20
		}
		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		where, args := "1=1", []any{}
		if req.ProductId > 0 {
			where += " AND c.product_id = ?"
			args = append(args, req.ProductId)
		}
		if req.Status >= 0 {
			where += " AND c.status = ?"
			args = append(args, req.Status)
		}
		if keyword := strings.TrimSpace(req.Keyword); keyword != "" {
			where += " AND (c.name LIKE ? OR p.name LIKE ?)"
			args = append(args, "%"+keyword+"%", "%"+keyword+"%")
		}
		var total int64
		if err := db.QueryRowContext(r.Context(), fmt.Sprintf(
			"SELECT COUNT(*) FROM mall_product.seckill_campaign c LEFT JOIN mall_product.product p ON p.id = c.product_id WHERE %s", where), args...).Scan(&total); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		args = append(args, req.PageSize, (req.Page-1)*req.PageSize)
		rows, err := db.QueryContext(r.Context(), fmt.Sprintf(
			`SELECT c.id, c.product_id, COALESCE(p.name,''), c.name, c.campaign_stock, c.per_user_limit,
			        COALESCE(c.starts_at,''), COALESCE(c.ends_at,''), c.status
			   FROM mall_product.seckill_campaign c
			   LEFT JOIN mall_product.product p ON p.id = c.product_id
			  WHERE %s ORDER BY c.id DESC LIMIT ? OFFSET ?`, where), args...)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer func() { _ = rows.Close() }()
		items := make([]types.AdminCampaignItem, 0)
		for rows.Next() {
			var item types.AdminCampaignItem
			if err := rows.Scan(&item.CampaignId, &item.ProductId, &item.ProductName, &item.Name, &item.CampaignStock,
				&item.PerUserLimit, &item.StartsAt, &item.EndsAt, &item.Status); err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			item.StatusText = campaignStatusText(item.Status)
			items = append(items, item)
		}
		httpx.OkJsonCtx(r.Context(), w, types.AdminCampaignListResp{Items: items, Total: total})
	}
}

func AdminCampaignUpsertHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminCampaignUpsertReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		if req.ProductId <= 0 || req.Name == "" || req.CampaignStock < 0 {
			writeBadRequest(w, "product_id, name and campaign_stock required")
			return
		}
		if req.PerUserLimit <= 0 {
			req.PerUserLimit = 1
		}
		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		campaignID := req.CampaignId
		if campaignID > 0 {
			if _, err := db.ExecContext(r.Context(),
				`UPDATE mall_product.seckill_campaign
				    SET product_id=?, name=?, campaign_stock=?, per_user_limit=?, starts_at=NULLIF(?,''), ends_at=NULLIF(?,''), status=?
				  WHERE id=?`,
				req.ProductId, req.Name, req.CampaignStock, req.PerUserLimit, req.StartsAt, req.EndsAt, req.Status, campaignID); err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
		} else {
			result, err := db.ExecContext(r.Context(),
				`INSERT INTO mall_product.seckill_campaign
				  (product_id, name, campaign_stock, per_user_limit, starts_at, ends_at, status)
				  VALUES (?, ?, ?, ?, NULLIF(?,''), NULLIF(?,''), ?)`,
				req.ProductId, req.Name, req.CampaignStock, req.PerUserLimit, req.StartsAt, req.EndsAt, req.Status)
			if err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			campaignID, _ = result.LastInsertId()
		}
		httpx.OkJsonCtx(r.Context(), w, types.AdminCampaignUpsertResp{CampaignId: campaignID})
	}
}

func campaignStatusText(status int64) string {
	switch status {
	case 0:
		return "draft"
	case 1:
		return "active"
	case 2:
		return "paused"
	case 3:
		return "ended"
	default:
		return "unknown"
	}
}
