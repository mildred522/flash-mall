package handler

import (
	"context"
	"database/sql"
	"fmt"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func AdminStockSnapshotRebuildHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req AdminStockSnapshotRebuildReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid stock snapshot rebuild request"))
			return
		}
		if req.ProductID < 0 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "product_id must be positive"))
			return
		}
		if req.Limit <= 0 || req.Limit > 10000 {
			req.Limit = 1000
		}
		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		if err := ensureGatewayProductReadTables(ctx, db); err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "snapshot schema unavailable", err))
			return
		}
		affected, err := rebuildGatewayStockSnapshots(ctx, db, req)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "stock snapshot rebuild failed", err))
			return
		}
		cardAffected, err := rebuildGatewayProductCardSnapshots(ctx, db, req.ProductID, req.Limit)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "product card snapshot rebuild failed", err))
			return
		}
		ok(ctx, c, map[string]any{"affected": affected, "card_affected": cardAffected, "limit": req.Limit})
	}
}

func rebuildGatewayStockSnapshots(ctx context.Context, db *sql.DB, req AdminStockSnapshotRebuildReq) (int64, error) {
	where, args := "1=1", []any{}
	if req.ProductID > 0 {
		where += " AND p.id = ?"
		args = append(args, req.ProductID)
	}
	args = append(args, req.Limit)
	result, err := db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO mall_product.product_stock_snapshot (product_id, available, reserved, total, source, version)
SELECT p.id, COALESCE(bucket.stock_available, 0), COALESCE(old.reserved, 0), COALESCE(bucket.stock_available, 0) + COALESCE(old.reserved, 0), 'admin-rebuild', 0
FROM mall_product.product p
LEFT JOIN (SELECT product_id, COALESCE(SUM(stock), 0) AS stock_available FROM mall_product.product_stock_bucket GROUP BY product_id) bucket ON bucket.product_id = p.id
LEFT JOIN mall_product.product_stock_snapshot old ON old.product_id = p.id
WHERE %s ORDER BY p.id LIMIT ?
ON DUPLICATE KEY UPDATE available=VALUES(available), reserved=VALUES(reserved), total=VALUES(total), source=VALUES(source), version=version+1`, where), args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
