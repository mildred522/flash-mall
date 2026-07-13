package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type supplierListQuery struct {
	Page     int64
	PageSize int64
	Status   int64
	Keyword  string
}

func AdminSupplierListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		req, appErr := parseSupplierListQuery(c)
		if appErr != nil {
			fail(ctx, c, consts.StatusBadRequest, appErr)
			return
		}

		items, total, err := loadAdminSuppliers(ctx, svcCtx, req)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin supplier query failed", err))
			return
		}
		ok(ctx, c, AdminSupplierListResp{Items: items, Total: total, Page: req.Page, PageSize: req.PageSize})
	}
}

func AdminSupplierDetailHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		supplierID, err := parsePositiveInt64(c.Query("supplier_id"))
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "supplier_id required"))
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin supplier detail query failed", err))
			return
		}
		item, err := loadAdminSupplierDetail(ctx, db, supplierID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeNotFound, "supplier not found"))
				return
			}
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin supplier detail query failed", err))
			return
		}
		ok(ctx, c, item)
	}
}

func AdminSupplierCreateHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req AdminSupplierCreateReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid supplier create request"))
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "name required"))
			return
		}
		if req.Status == 0 {
			req.Status = 1
		}
		if req.Status != 1 && req.Status != 2 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "status must be 1 or 2"))
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin supplier create failed", err))
			return
		}
		result, err := db.ExecContext(ctx, "INSERT INTO mall_product.supplier (name, status) VALUES (?, ?)", req.Name, req.Status)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin supplier create failed", err))
			return
		}
		supplierID, _ := result.LastInsertId()
		recordGatewayAdminAuditEvent(c, svcCtx, adminAuditSupplierCreated, fmt.Sprintf("supplier:%d name:%s", supplierID, req.Name))
		ok(ctx, c, AdminSupplierCreateResp{SupplierID: supplierID})
	}
}

func AdminSupplierUpdateHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req AdminSupplierUpdateReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid supplier update request"))
			return
		}
		if req.SupplierID <= 0 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "supplier_id required"))
			return
		}

		setClauses := make([]string, 0, 2)
		args := make([]any, 0, 3)
		if name := strings.TrimSpace(req.Name); name != "" {
			setClauses = append(setClauses, "name = ?")
			args = append(args, name)
		}
		if req.Status != nil {
			if *req.Status != 1 && *req.Status != 2 {
				fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "status must be 1 or 2"))
				return
			}
			setClauses = append(setClauses, "status = ?")
			args = append(args, *req.Status)
		}
		if len(setClauses) == 0 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "no fields to update"))
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin supplier update failed", err))
			return
		}
		if req.Status != nil && *req.Status == 2 {
			activeProducts, err := countActiveSupplierProducts(ctx, db, req.SupplierID)
			if err != nil {
				fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin supplier update failed", err))
				return
			}
			if activeProducts > 0 {
				recordGatewayAdminAuditFailure(c, svcCtx, supplierUpdateAuditEvent(req.Status), fmt.Sprintf("supplier:%d reason:%s", req.SupplierID, adminAuditReasonHasActiveProducts))
				fail(ctx, c, consts.StatusConflict, apperror.New(apperror.CodeConflict, "supplier has active products"))
				return
			}
		}

		query := fmt.Sprintf("UPDATE mall_product.supplier SET %s WHERE id = ?", strings.Join(setClauses, ", "))
		args = append(args, req.SupplierID)
		result, err := db.ExecContext(ctx, query, args...)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin supplier update failed", err))
			return
		}
		rows, err := result.RowsAffected()
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin supplier update failed", err))
			return
		}
		if rows == 0 {
			exists, err := supplierExists(ctx, db, req.SupplierID)
			if err != nil {
				fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin supplier update failed", err))
				return
			}
			if !exists {
				recordGatewayAdminAuditFailure(c, svcCtx, supplierUpdateAuditEvent(req.Status), fmt.Sprintf("supplier:%d reason:%s", req.SupplierID, adminAuditReasonNotFound))
				fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeNotFound, "supplier not found"))
				return
			}
		}
		recordGatewayAdminAuditEvent(c, svcCtx, supplierUpdateAuditEvent(req.Status), fmt.Sprintf("supplier:%d", req.SupplierID))
		ok(ctx, c, map[string]any{"ok": true})
	}
}

func parseSupplierListQuery(c *app.RequestContext) (supplierListQuery, *apperror.Error) {
	page, err := parseInt64Default(c.Query("page"), defaultProductPage)
	if err != nil || page <= 0 {
		return supplierListQuery{}, apperror.New(apperror.CodeInvalidArgument, "page must be positive")
	}
	pageSize, err := parseInt64Default(c.Query("page_size"), defaultProductPageSize)
	if err != nil || pageSize <= 0 {
		return supplierListQuery{}, apperror.New(apperror.CodeInvalidArgument, "page_size must be positive")
	}
	if pageSize > maxProductPageSize {
		pageSize = maxProductPageSize
	}
	status, err := parseInt64Default(c.Query("status"), -1)
	if err != nil {
		return supplierListQuery{}, apperror.New(apperror.CodeInvalidArgument, "status must be numeric")
	}
	return supplierListQuery{
		Page:     page,
		PageSize: pageSize,
		Status:   status,
		Keyword:  strings.TrimSpace(c.Query("keyword")),
	}, nil
}

func loadAdminSuppliers(ctx context.Context, svcCtx *svc.ServiceContext, req supplierListQuery) ([]AdminSupplierItem, int64, error) {
	db, err := svcCtx.SqlConn.RawDB()
	if err != nil {
		return nil, 0, err
	}

	where, args := supplierWhereClause(req)
	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM mall_product.supplier s WHERE %s", where)
	if err := db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []AdminSupplierItem{}, 0, nil
	}

	offset := (req.Page - 1) * req.PageSize
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
	rows, err := db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]AdminSupplierItem, 0, req.PageSize)
	for rows.Next() {
		var item AdminSupplierItem
		if err := rows.Scan(&item.SupplierID, &item.Name, &item.Status, &item.ProductCount, &item.ActiveProducts); err != nil {
			return nil, 0, err
		}
		item.StatusText = supplierStatusText(item.Status)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func loadAdminSupplierDetail(ctx context.Context, db *sql.DB, supplierID int64) (AdminSupplierItem, error) {
	var item AdminSupplierItem
	err := db.QueryRowContext(ctx, `SELECT s.id, s.name, s.status,
COALESCE(stats.product_count, 0), COALESCE(stats.active_products, 0)
FROM mall_product.supplier s
LEFT JOIN (
  SELECT supplier_id, COUNT(*) AS product_count, SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END) AS active_products
  FROM mall_product.product
  GROUP BY supplier_id
) stats ON stats.supplier_id = s.id
WHERE s.id = ?`, supplierID).Scan(&item.SupplierID, &item.Name, &item.Status, &item.ProductCount, &item.ActiveProducts)
	if err != nil {
		return AdminSupplierItem{}, err
	}
	item.StatusText = supplierStatusText(item.Status)
	return item, nil
}

func supplierWhereClause(req supplierListQuery) (string, []any) {
	where := "1=1"
	args := make([]any, 0, 2)
	if req.Status >= 0 {
		where += " AND s.status = ?"
		args = append(args, req.Status)
	}
	if req.Keyword != "" {
		where += " AND s.name LIKE ?"
		args = append(args, "%"+req.Keyword+"%")
	}
	return where, args
}

func countActiveSupplierProducts(ctx context.Context, db *sql.DB, supplierID int64) (int64, error) {
	var activeProducts int64
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mall_product.product WHERE supplier_id = ? AND status = 1", supplierID).Scan(&activeProducts)
	return activeProducts, err
}

func supplierExists(ctx context.Context, db *sql.DB, supplierID int64) (bool, error) {
	var exists int64
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mall_product.supplier WHERE id = ?", supplierID).Scan(&exists); err != nil {
		return false, err
	}
	return exists > 0, nil
}

func supplierStatusText(status int64) string {
	switch status {
	case 1:
		return "active"
	case 2:
		return "inactive"
	default:
		return "unknown"
	}
}
