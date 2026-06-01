package handler

import (
	"fmt"
	"net/http"

	"flash-mall/app/order/api/internal/svc"
	"flash-mall/app/order/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func AdminProductListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminProductListReq
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
		offset := (req.Page - 1) * req.PageSize

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		var total int64
		_ = db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM product").Scan(&total)

		rows, err := db.QueryContext(r.Context(),
			`SELECT p.id, p.name, p.origin_price_fen, p.sale_price_fen, COALESCE(SUM(b.stock), 0), p.status
			 FROM product p LEFT JOIN product_stock_bucket b ON b.product_id = p.id
			 GROUP BY p.id, p.name, p.origin_price_fen, p.sale_price_fen, p.status
			 ORDER BY p.id LIMIT ? OFFSET ?`, req.PageSize, offset)
		if err != nil {
			logx.WithContext(r.Context()).Errorf("admin product list query failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer rows.Close()

		items := make([]types.AdminProductItem, 0)
		for rows.Next() {
			var item types.AdminProductItem
			if err := rows.Scan(&item.ProductId, &item.Name, &item.OriginPriceFen, &item.SalePriceFen, &item.StockAvailable, &item.Status); err != nil {
				continue
			}
			items = append(items, item)
		}

		httpx.OkJsonCtx(r.Context(), w, types.AdminProductListResp{Items: items, Total: total})
	}
}

func AdminProductUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ProductId      int64  `json:"product_id"`
			Name           string `json:"name,optional"`
			SalePriceFen   int64  `json:"sale_price_fen,optional"`
			OriginPriceFen int64  `json:"origin_price_fen,optional"`
			Status         int64  `json:"status,optional"`
		}
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if req.ProductId == 0 {
			httpx.OkJsonCtx(r.Context(), w, map[string]any{"error": "product_id required"})
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		setClauses := ""
		args := []interface{}{}
		if req.Name != "" {
			setClauses += "name = ?"
			args = append(args, req.Name)
		}
		if req.SalePriceFen > 0 {
			if setClauses != "" {
				setClauses += ", "
			}
			setClauses += "sale_price_fen = ?"
			args = append(args, req.SalePriceFen)
		}
		if req.OriginPriceFen > 0 {
			if setClauses != "" {
				setClauses += ", "
			}
			setClauses += "origin_price_fen = ?"
			args = append(args, req.OriginPriceFen)
		}
		if setClauses != "" {
			if setClauses != "" {
				setClauses += ", "
			}
			setClauses += "status = ?"
			args = append(args, req.Status)
		}

		if setClauses == "" {
			httpx.OkJsonCtx(r.Context(), w, map[string]any{"error": "no fields to update"})
			return
		}

		query := fmt.Sprintf("UPDATE product SET %s WHERE id = ?", setClauses)
		args = append(args, req.ProductId)
		_, err = db.ExecContext(r.Context(), query, args...)
		if err != nil {
			logx.WithContext(r.Context()).Errorf("admin product update failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		httpx.OkJsonCtx(r.Context(), w, map[string]any{"ok": true})
	}
}
