package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"flash-mall/app/entry/api/internal/svc"
	"flash-mall/app/entry/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func AdminSupplierListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminSupplierListReq
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

		where := "1=1"
		args := []any{}
		if req.Status >= 0 {
			where += " AND s.status = ?"
			args = append(args, req.Status)
		}
		if keyword := strings.TrimSpace(req.Keyword); keyword != "" {
			where += " AND s.name LIKE ?"
			args = append(args, "%"+keyword+"%")
		}

		var total int64
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM mall_product.supplier s WHERE %s", where)
		if err := db.QueryRowContext(r.Context(), countQuery, args...).Scan(&total); err != nil {
			logx.WithContext(r.Context()).Errorf("admin supplier count query failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		query := fmt.Sprintf(`SELECT s.id, s.name, s.status,
COALESCE(stats.product_count, 0), COALESCE(stats.active_products, 0)
FROM mall_product.supplier s
LEFT JOIN (
  SELECT supplier_id, COUNT(*) AS product_count, SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END) AS active_products
  FROM mall_product.product
  GROUP BY supplier_id
) stats ON stats.supplier_id = s.id
WHERE %s
ORDER BY s.id DESC LIMIT ? OFFSET ?`, where)
		queryArgs := append(append([]any{}, args...), req.PageSize, offset)
		rows, err := db.QueryContext(r.Context(), query, queryArgs...)
		if err != nil {
			logx.WithContext(r.Context()).Errorf("admin supplier list query failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer func() { _ = rows.Close() }()

		items := make([]types.AdminSupplierItem, 0)
		for rows.Next() {
			var item types.AdminSupplierItem
			if err := rows.Scan(&item.SupplierId, &item.Name, &item.Status, &item.ProductCount, &item.ActiveProducts); err != nil {
				logx.WithContext(r.Context()).Errorf("admin supplier list scan failed: %v", err)
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			item.StatusText = adminSupplierStatusText(item.Status)
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			logx.WithContext(r.Context()).Errorf("admin supplier list rows failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, types.AdminSupplierListResp{Items: items, Total: total})
	}
}

func AdminSupplierDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminSupplierDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if req.SupplierId <= 0 {
			writeBadRequest(w, "supplier_id required")
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		item, err := adminSupplierDetail(r.Context(), db, req.SupplierId)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeNotFound(w, "supplier not found")
				return
			}
			logx.WithContext(r.Context()).Errorf("admin supplier detail query failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, item)
	}
}

func AdminSupplierCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminSupplierCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" {
			writeBadRequest(w, "name required")
			return
		}
		if req.Status == 0 {
			req.Status = 1
		}
		if req.Status != 1 && req.Status != 2 {
			writeBadRequest(w, "status must be 1 or 2")
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		result, err := db.ExecContext(r.Context(), "INSERT INTO mall_product.supplier (name, status) VALUES (?, ?)", req.Name, req.Status)
		if err != nil {
			logx.WithContext(r.Context()).Errorf("admin supplier create failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		supplierID, _ := result.LastInsertId()
		recordAdminAuditEvent(r, svcCtx, adminAuditSupplierCreated, fmt.Sprintf("supplier:%d name:%s", supplierID, req.Name))
		httpx.OkJsonCtx(r.Context(), w, types.AdminSupplierCreateResp{SupplierId: supplierID})
	}
}

func AdminSupplierUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminSupplierUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if req.SupplierId <= 0 {
			writeBadRequest(w, "supplier_id required")
			return
		}

		setClauses := ""
		args := []any{}
		if name := strings.TrimSpace(req.Name); name != "" {
			setClauses += "name = ?"
			args = append(args, name)
		}
		if req.Status != nil {
			if *req.Status != 1 && *req.Status != 2 {
				writeBadRequest(w, "status must be 1 or 2")
				return
			}
			if setClauses != "" {
				setClauses += ", "
			}
			setClauses += "status = ?"
			args = append(args, *req.Status)
		}
		if setClauses == "" {
			writeBadRequest(w, "no fields to update")
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if req.Status != nil && *req.Status == 2 {
			var activeProducts int64
			if err := db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM mall_product.product WHERE supplier_id = ? AND status = 1", req.SupplierId).Scan(&activeProducts); err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			if activeProducts > 0 {
				recordAdminAuditFailure(r, svcCtx, adminSupplierUpdateAuditEvent(req.Status), fmt.Sprintf("supplier:%d reason:%s", req.SupplierId, adminAuditReasonHasActiveProducts))
				writeConflict(w, "supplier has active products")
				return
			}
		}

		query := fmt.Sprintf("UPDATE mall_product.supplier SET %s WHERE id = ?", setClauses)
		args = append(args, req.SupplierId)
		result, err := db.ExecContext(r.Context(), query, args...)
		if err != nil {
			logx.WithContext(r.Context()).Errorf("admin supplier update failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		rows, err := result.RowsAffected()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if rows == 0 {
			var exists int64
			if err := db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM mall_product.supplier WHERE id = ?", req.SupplierId).Scan(&exists); err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			if exists == 0 {
				recordAdminAuditFailure(r, svcCtx, adminSupplierUpdateAuditEvent(req.Status), fmt.Sprintf("supplier:%d reason:%s", req.SupplierId, adminAuditReasonNotFound))
				writeNotFound(w, "supplier not found")
				return
			}
		}
		recordAdminAuditEvent(r, svcCtx, adminSupplierUpdateAuditEvent(req.Status), fmt.Sprintf("supplier:%d", req.SupplierId))
		httpx.OkJsonCtx(r.Context(), w, map[string]any{"ok": true})
	}
}

type adminQueryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func adminSupplierDetail(ctx context.Context, q adminQueryRower, supplierID int64) (types.AdminSupplierItem, error) {
	var item types.AdminSupplierItem
	err := q.QueryRowContext(ctx, `SELECT s.id, s.name, s.status,
COALESCE(stats.product_count, 0), COALESCE(stats.active_products, 0)
FROM mall_product.supplier s
LEFT JOIN (
  SELECT supplier_id, COUNT(*) AS product_count, SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END) AS active_products
  FROM mall_product.product
  GROUP BY supplier_id
) stats ON stats.supplier_id = s.id
WHERE s.id = ?`, supplierID).Scan(&item.SupplierId, &item.Name, &item.Status, &item.ProductCount, &item.ActiveProducts)
	if err != nil {
		return types.AdminSupplierItem{}, err
	}
	item.StatusText = adminSupplierStatusText(item.Status)
	return item, nil
}

func adminActiveSupplierExists(ctx context.Context, q adminQueryRower, supplierID int64) (bool, error) {
	var exists int64
	if err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM mall_product.supplier WHERE id = ? AND status = 1", supplierID).Scan(&exists); err != nil {
		return false, err
	}
	return exists > 0, nil
}

func adminSupplierStatusText(status int64) string {
	switch status {
	case 1:
		return "active"
	case 2:
		return "inactive"
	default:
		return "unknown"
	}
}

func adminSupplierUpdateAuditEvent(status *int64) string {
	if status == nil {
		return adminAuditSupplierUpdated
	}
	if *status == 1 {
		return adminAuditSupplierEnabled
	}
	return adminAuditSupplierDisabled
}
