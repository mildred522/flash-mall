package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

const adminPromotionTypeLimitedPrice = "LIMITED_PRICE"

type promotionListQuery struct {
	Page         int64
	PageSize     int64
	ProductID    int64
	Status       int64
	EffectStatus string
	Keyword      string
}

func AdminPromotionListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		req, appErr := parsePromotionListQuery(c)
		if appErr != nil {
			fail(ctx, c, consts.StatusBadRequest, appErr)
			return
		}

		items, total, err := loadAdminPromotions(ctx, svcCtx, req)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin promotion query failed", err))
			return
		}
		ok(ctx, c, AdminPromotionListResp{Items: items, Total: total, Page: req.Page, PageSize: req.PageSize})
	}
}

func AdminPromotionDetailHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		promotionID, err := parsePositiveInt64(c.Query("promotion_id"))
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "promotion_id required"))
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin promotion detail query failed", err))
			return
		}
		item, err := loadAdminPromotionDetail(ctx, db, promotionID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeNotFound, "promotion not found"))
				return
			}
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin promotion detail query failed", err))
			return
		}
		ok(ctx, c, item)
	}
}

func AdminPromotionCreateHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req AdminPromotionCreateReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid promotion create request"))
			return
		}
		req.Type = normalizePromotionType(req.Type)
		if req.ProductID <= 0 || req.DiscountValue <= 0 || req.ThresholdAmount < 0 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "product_id, positive discount_value and non-negative threshold_amount are required"))
			return
		}
		if req.Type != adminPromotionTypeLimitedPrice {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "unsupported promotion type"))
			return
		}
		if req.Status == 0 {
			req.Status = 1
		}
		if req.Status != 1 && req.Status != 2 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "status must be 1 or 2"))
			return
		}
		startsAt, endsAt, appErr := parsePromotionWindow(req.StartsAt, req.EndsAt)
		if appErr != nil {
			fail(ctx, c, consts.StatusBadRequest, appErr)
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin promotion create failed", err))
			return
		}
		exists, err := productExists(ctx, db, req.ProductID)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin promotion create failed", err))
			return
		}
		if !exists {
			recordGatewayAdminAuditFailure(c, svcCtx, adminAuditPromotionCreated, fmt.Sprintf("product:%d reason:%s", req.ProductID, adminAuditReasonProductNotFound))
			fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeProductNotFound, "product not found"))
			return
		}
		validDiscount, err := promotionDiscountWithinSale(ctx, db, req.ProductID, req.DiscountValue)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin promotion create failed", err))
			return
		}
		if !validDiscount {
			recordGatewayAdminAuditFailure(c, svcCtx, adminAuditPromotionCreated, fmt.Sprintf("product:%d reason:%s", req.ProductID, adminAuditReasonInvalidDiscount))
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "discount_value must be <= product sale_price_fen"))
			return
		}
		if req.Status == 1 {
			conflict, err := promotionHasActiveConflict(ctx, db, req.ProductID, 0, startsAt, endsAt)
			if err != nil {
				fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin promotion create failed", err))
				return
			}
			if conflict {
				recordGatewayAdminAuditFailure(c, svcCtx, adminAuditPromotionCreated, fmt.Sprintf("product:%d reason:%s", req.ProductID, adminAuditReasonWindowConflict))
				fail(ctx, c, consts.StatusConflict, apperror.New(apperror.CodeConflict, "active limited price promotion window overlaps"))
				return
			}
		}

		result, err := db.ExecContext(ctx, `
INSERT INTO mall_product.promotion_rule (product_id, type, discount_value, threshold_amount, starts_at, ends_at, status)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
			req.ProductID, req.Type, req.DiscountValue, req.ThresholdAmount, startsAt, endsAt, req.Status,
		)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin promotion create failed", err))
			return
		}
		promotionID, _ := result.LastInsertId()
		refreshProductCardSnapshotsBestEffort(ctx, svcCtx, req.ProductID)
		recordGatewayAdminAuditEvent(c, svcCtx, adminAuditPromotionCreated, fmt.Sprintf("promotion:%d product:%d", promotionID, req.ProductID))
		ok(ctx, c, AdminPromotionCreateResp{PromotionID: promotionID})
	}
}

func AdminPromotionUpdateHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req AdminPromotionUpdateReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid promotion update request"))
			return
		}
		if req.PromotionID <= 0 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "promotion_id required"))
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin promotion update failed", err))
			return
		}
		previousProductIDs := promotionProductIDsBestEffort(ctx, db, req.PromotionID)

		setClauses := make([]string, 0, 6)
		args := make([]any, 0, 7)
		var parsedStartsAt any
		var parsedEndsAt any
		if req.ProductID != nil {
			if *req.ProductID <= 0 {
				fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "product_id required"))
				return
			}
			exists, err := productExists(ctx, db, *req.ProductID)
			if err != nil {
				fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin promotion update failed", err))
				return
			}
			if !exists {
				recordGatewayAdminAuditFailure(c, svcCtx, promotionUpdateAuditEvent(req.Status), fmt.Sprintf("promotion:%d product:%d reason:%s", req.PromotionID, *req.ProductID, adminAuditReasonProductNotFound))
				fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeProductNotFound, "product not found"))
				return
			}
			setClauses = append(setClauses, "product_id = ?")
			args = append(args, *req.ProductID)
		}
		if req.DiscountValue != nil {
			if *req.DiscountValue <= 0 {
				fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "discount_value must be positive"))
				return
			}
			setClauses = append(setClauses, "discount_value = ?")
			args = append(args, *req.DiscountValue)
		}
		if req.ThresholdAmount != nil {
			if *req.ThresholdAmount < 0 {
				fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "threshold_amount must be non-negative"))
				return
			}
			setClauses = append(setClauses, "threshold_amount = ?")
			args = append(args, *req.ThresholdAmount)
		}
		if req.StartsAt != nil {
			startsAt, parseErr := parsePromotionTime(*req.StartsAt)
			if parseErr != nil {
				fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, parseErr.Error()))
				return
			}
			parsedStartsAt = startsAt
			setClauses = append(setClauses, "starts_at = ?")
			args = append(args, startsAt)
		}
		if req.EndsAt != nil {
			endsAt, parseErr := parsePromotionTime(*req.EndsAt)
			if parseErr != nil {
				fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, parseErr.Error()))
				return
			}
			parsedEndsAt = endsAt
			setClauses = append(setClauses, "ends_at = ?")
			args = append(args, endsAt)
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

		if req.ProductID != nil || req.DiscountValue != nil {
			finalProductID, finalDiscountValue, found, err := promotionProductDiscount(ctx, db, req.PromotionID)
			if err != nil {
				fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin promotion update failed", err))
				return
			}
			if !found {
				recordGatewayAdminAuditFailure(c, svcCtx, promotionUpdateAuditEvent(req.Status), fmt.Sprintf("promotion:%d reason:%s", req.PromotionID, adminAuditReasonNotFound))
				fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeNotFound, "promotion not found"))
				return
			}
			if req.ProductID != nil {
				finalProductID = *req.ProductID
			}
			if req.DiscountValue != nil {
				finalDiscountValue = *req.DiscountValue
			}
			validDiscount, err := promotionDiscountWithinSale(ctx, db, finalProductID, finalDiscountValue)
			if err != nil {
				fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin promotion update failed", err))
				return
			}
			if !validDiscount {
				recordGatewayAdminAuditFailure(c, svcCtx, promotionUpdateAuditEvent(req.Status), fmt.Sprintf("promotion:%d reason:%s", req.PromotionID, adminAuditReasonInvalidDiscount))
				fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "discount_value must be <= product sale_price_fen"))
				return
			}
		}

		var currentStartsAt, currentEndsAt sql.NullTime
		hasCurrentWindow := false
		if req.ProductID != nil || req.Status != nil || req.StartsAt != nil || req.EndsAt != nil {
			finalProductID, finalStatus, found, err := promotionProductStatusWindow(ctx, db, req.PromotionID, &currentStartsAt, &currentEndsAt)
			if err != nil {
				fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin promotion update failed", err))
				return
			}
			if !found {
				recordGatewayAdminAuditFailure(c, svcCtx, promotionUpdateAuditEvent(req.Status), fmt.Sprintf("promotion:%d reason:%s", req.PromotionID, adminAuditReasonNotFound))
				fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeNotFound, "promotion not found"))
				return
			}
			hasCurrentWindow = true
			if req.ProductID != nil {
				finalProductID = *req.ProductID
			}
			if req.Status != nil {
				finalStatus = *req.Status
			}
			if finalStatus == 1 {
				finalStartsAt := promotionConflictBound(currentStartsAt, parsedStartsAt, req.StartsAt != nil)
				finalEndsAt := promotionConflictBound(currentEndsAt, parsedEndsAt, req.EndsAt != nil)
				conflict, err := promotionHasActiveConflict(ctx, db, finalProductID, req.PromotionID, finalStartsAt, finalEndsAt)
				if err != nil {
					fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin promotion update failed", err))
					return
				}
				if conflict {
					recordGatewayAdminAuditFailure(c, svcCtx, promotionUpdateAuditEvent(req.Status), fmt.Sprintf("promotion:%d reason:%s", req.PromotionID, adminAuditReasonWindowConflict))
					fail(ctx, c, consts.StatusConflict, apperror.New(apperror.CodeConflict, "active limited price promotion window overlaps"))
					return
				}
			}
		}
		if req.StartsAt != nil || req.EndsAt != nil {
			if !hasCurrentWindow {
				err := db.QueryRowContext(ctx, "SELECT starts_at, ends_at FROM mall_product.promotion_rule WHERE id = ?", req.PromotionID).Scan(&currentStartsAt, &currentEndsAt)
				if errors.Is(err, sql.ErrNoRows) {
					recordGatewayAdminAuditFailure(c, svcCtx, promotionUpdateAuditEvent(req.Status), fmt.Sprintf("promotion:%d reason:%s", req.PromotionID, adminAuditReasonNotFound))
					fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeNotFound, "promotion not found"))
					return
				}
				if err != nil {
					fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin promotion update failed", err))
					return
				}
			}
			if !promotionWindowIsValid(currentStartsAt, currentEndsAt, parsedStartsAt, req.StartsAt != nil, parsedEndsAt, req.EndsAt != nil) {
				recordGatewayAdminAuditFailure(c, svcCtx, promotionUpdateAuditEvent(req.Status), fmt.Sprintf("promotion:%d reason:%s", req.PromotionID, adminAuditReasonInvalidWindow))
				fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "ends_at must be after starts_at"))
				return
			}
		}

		query := fmt.Sprintf("UPDATE mall_product.promotion_rule SET %s WHERE id = ?", strings.Join(setClauses, ", "))
		args = append(args, req.PromotionID)
		result, err := db.ExecContext(ctx, query, args...)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin promotion update failed", err))
			return
		}
		rows, err := result.RowsAffected()
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin promotion update failed", err))
			return
		}
		if rows == 0 {
			exists, err := promotionExists(ctx, db, req.PromotionID)
			if err != nil {
				fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin promotion update failed", err))
				return
			}
			if !exists {
				recordGatewayAdminAuditFailure(c, svcCtx, promotionUpdateAuditEvent(req.Status), fmt.Sprintf("promotion:%d reason:%s", req.PromotionID, adminAuditReasonNotFound))
				fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeNotFound, "promotion not found"))
				return
			}
		}
		updatedProductIDs := promotionProductIDsBestEffort(ctx, db, req.PromotionID)
		refreshProductCardSnapshotsBestEffort(ctx, svcCtx, append(previousProductIDs, updatedProductIDs...)...)
		recordGatewayAdminAuditEvent(c, svcCtx, promotionUpdateAuditEvent(req.Status), fmt.Sprintf("promotion:%d", req.PromotionID))
		ok(ctx, c, map[string]any{"ok": true})
	}
}

func parsePromotionListQuery(c *app.RequestContext) (promotionListQuery, *apperror.Error) {
	page, err := parseInt64Default(c.Query("page"), defaultProductPage)
	if err != nil || page <= 0 {
		return promotionListQuery{}, apperror.New(apperror.CodeInvalidArgument, "page must be positive")
	}
	pageSize, err := parseInt64Default(c.Query("page_size"), defaultProductPageSize)
	if err != nil || pageSize <= 0 {
		return promotionListQuery{}, apperror.New(apperror.CodeInvalidArgument, "page_size must be positive")
	}
	if pageSize > maxProductPageSize {
		pageSize = maxProductPageSize
	}
	productID, err := parseOptionalInt64(c.Query("product_id"))
	if err != nil {
		return promotionListQuery{}, apperror.New(apperror.CodeInvalidArgument, "product_id must be numeric")
	}
	status, err := parseInt64Default(c.Query("status"), -1)
	if err != nil {
		return promotionListQuery{}, apperror.New(apperror.CodeInvalidArgument, "status must be numeric")
	}
	return promotionListQuery{
		Page:         page,
		PageSize:     pageSize,
		ProductID:    productID,
		Status:       status,
		EffectStatus: strings.TrimSpace(c.Query("effect_status")),
		Keyword:      strings.TrimSpace(c.Query("keyword")),
	}, nil
}

func loadAdminPromotions(ctx context.Context, svcCtx *svc.ServiceContext, req promotionListQuery) ([]AdminPromotionItem, int64, error) {
	db, err := svcCtx.SqlConn.RawDB()
	if err != nil {
		return nil, 0, err
	}

	where, args := promotionWhereClause(req)
	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM mall_product.promotion_rule pr LEFT JOIN mall_product.product p ON p.id = pr.product_id WHERE %s", where)
	if err := db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []AdminPromotionItem{}, 0, nil
	}

	offset := (req.Page - 1) * req.PageSize
	query := fmt.Sprintf(`
SELECT pr.id, pr.product_id, COALESCE(p.name, ''), COALESCE(p.origin_price_fen, 0), COALESCE(p.sale_price_fen, 0),
       pr.type, pr.discount_value, pr.threshold_amount, pr.starts_at, pr.ends_at, pr.status
FROM mall_product.promotion_rule pr
LEFT JOIN mall_product.product p ON p.id = pr.product_id
WHERE %s
ORDER BY pr.id DESC
LIMIT ? OFFSET ?`, where)
	queryArgs := append(append([]any{}, args...), req.PageSize, offset)
	rows, err := db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]AdminPromotionItem, 0, req.PageSize)
	for rows.Next() {
		item, err := scanAdminPromotionItem(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func loadAdminPromotionDetail(ctx context.Context, db *sql.DB, promotionID int64) (AdminPromotionItem, error) {
	row := db.QueryRowContext(ctx, `
SELECT pr.id, pr.product_id, COALESCE(p.name, ''), COALESCE(p.origin_price_fen, 0), COALESCE(p.sale_price_fen, 0),
       pr.type, pr.discount_value, pr.threshold_amount, pr.starts_at, pr.ends_at, pr.status
FROM mall_product.promotion_rule pr
LEFT JOIN mall_product.product p ON p.id = pr.product_id
WHERE pr.id = ?`, promotionID)
	return scanAdminPromotionItem(row)
}

func promotionWhereClause(req promotionListQuery) (string, []any) {
	where := "1=1"
	args := make([]any, 0, 3)
	if req.ProductID > 0 {
		where += " AND pr.product_id = ?"
		args = append(args, req.ProductID)
	}
	if req.Status >= 0 {
		where += " AND pr.status = ?"
		args = append(args, req.Status)
	}
	switch strings.TrimSpace(req.EffectStatus) {
	case "active":
		where += " AND pr.status = 1 AND (pr.starts_at IS NULL OR pr.starts_at <= NOW()) AND (pr.ends_at IS NULL OR pr.ends_at >= NOW())"
	case "scheduled":
		where += " AND pr.status = 1 AND pr.starts_at IS NOT NULL AND pr.starts_at > NOW()"
	case "expired":
		where += " AND pr.status = 1 AND pr.ends_at IS NOT NULL AND pr.ends_at < NOW()"
	case "inactive":
		where += " AND pr.status <> 1"
	}
	if req.Keyword != "" {
		where += " AND p.name LIKE ?"
		args = append(args, "%"+req.Keyword+"%")
	}
	return where, args
}

type promotionScanner interface {
	Scan(dest ...any) error
}

func scanAdminPromotionItem(scanner promotionScanner) (AdminPromotionItem, error) {
	var item AdminPromotionItem
	var startsAt, endsAt sql.NullTime
	if err := scanner.Scan(
		&item.PromotionID,
		&item.ProductID,
		&item.ProductName,
		&item.OriginPriceFen,
		&item.SalePriceFen,
		&item.Type,
		&item.DiscountValue,
		&item.ThresholdAmount,
		&startsAt,
		&endsAt,
		&item.Status,
	); err != nil {
		return AdminPromotionItem{}, err
	}
	item.StartsAt = formatPromotionTime(startsAt)
	item.EndsAt = formatPromotionTime(endsAt)
	item.StatusText = promotionStatusText(item.Status)
	item.EffectStatus, item.EffectStatusText = promotionEffectStatus(item.Status, startsAt, endsAt)
	return item, nil
}

func formatPromotionTime(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format("2006-01-02 15:04:05")
}

func promotionStatusText(status int64) string {
	switch status {
	case 1:
		return "active"
	case 2:
		return "inactive"
	default:
		return "unknown"
	}
}

func promotionEffectStatus(status int64, startsAt, endsAt sql.NullTime) (string, string) {
	if status != 1 {
		return "inactive", "停用"
	}
	now := time.Now()
	if startsAt.Valid && startsAt.Time.After(now) {
		return "scheduled", "未开始"
	}
	if endsAt.Valid && endsAt.Time.Before(now) {
		return "expired", "已结束"
	}
	return "active", "生效中"
}

func normalizePromotionType(value string) string {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return adminPromotionTypeLimitedPrice
	}
	return value
}

func parsePromotionWindow(startsAtValue, endsAtValue string) (any, any, *apperror.Error) {
	startsAt, err := parsePromotionTime(startsAtValue)
	if err != nil {
		return nil, nil, apperror.New(apperror.CodeInvalidArgument, err.Error())
	}
	endsAt, err := parsePromotionTime(endsAtValue)
	if err != nil {
		return nil, nil, apperror.New(apperror.CodeInvalidArgument, err.Error())
	}
	startTime, startOK := startsAt.(time.Time)
	endTime, endOK := endsAt.(time.Time)
	if startOK && endOK && endTime.Before(startTime) {
		return nil, nil, apperror.New(apperror.CodeInvalidArgument, "ends_at must be after starts_at")
	}
	return startsAt, endsAt, nil
}

func parsePromotionTime(value string) (any, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed, nil
		}
	}
	return nil, fmt.Errorf("invalid time: %s", value)
}

func promotionWindowIsValid(currentStartsAt, currentEndsAt sql.NullTime, nextStartsAt any, hasNextStartsAt bool, nextEndsAt any, hasNextEndsAt bool) bool {
	startsAt := promotionEffectiveTime(currentStartsAt, nextStartsAt, hasNextStartsAt)
	endsAt := promotionEffectiveTime(currentEndsAt, nextEndsAt, hasNextEndsAt)
	return startsAt == nil || endsAt == nil || !endsAt.Before(*startsAt)
}

func promotionEffectiveTime(current sql.NullTime, next any, hasNext bool) *time.Time {
	if hasNext {
		if parsed, ok := next.(time.Time); ok {
			return &parsed
		}
		return nil
	}
	if current.Valid {
		value := current.Time
		return &value
	}
	return nil
}

func productExists(ctx context.Context, db *sql.DB, productID int64) (bool, error) {
	var exists int64
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mall_product.product WHERE id = ?", productID).Scan(&exists); err != nil {
		return false, err
	}
	return exists > 0, nil
}

func promotionExists(ctx context.Context, db *sql.DB, promotionID int64) (bool, error) {
	var exists int64
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mall_product.promotion_rule WHERE id = ?", promotionID).Scan(&exists); err != nil {
		return false, err
	}
	return exists > 0, nil
}

func promotionProductStatusWindow(ctx context.Context, db *sql.DB, promotionID int64, startsAt, endsAt *sql.NullTime) (int64, int64, bool, error) {
	var productID int64
	var status int64
	err := db.QueryRowContext(ctx, "SELECT product_id, status, starts_at, ends_at FROM mall_product.promotion_rule WHERE id = ?", promotionID).Scan(&productID, &status, startsAt, endsAt)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, err
	}
	return productID, status, true, nil
}

func promotionProductDiscount(ctx context.Context, db *sql.DB, promotionID int64) (int64, int64, bool, error) {
	var productID int64
	var discountValue int64
	err := db.QueryRowContext(ctx, "SELECT product_id, discount_value FROM mall_product.promotion_rule WHERE id = ?", promotionID).Scan(&productID, &discountValue)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, err
	}
	return productID, discountValue, true, nil
}

func promotionDiscountWithinSale(ctx context.Context, db *sql.DB, productID, discountValue int64) (bool, error) {
	var salePriceFen int64
	err := db.QueryRowContext(ctx, "SELECT sale_price_fen FROM mall_product.product WHERE id = ?", productID).Scan(&salePriceFen)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return discountValue <= salePriceFen, nil
}

func promotionHasActiveConflict(ctx context.Context, db *sql.DB, productID, excludePromotionID int64, startsAt, endsAt any) (bool, error) {
	var exists int64
	err := db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM mall_product.promotion_rule
WHERE product_id = ?
  AND type = ?
  AND status = 1
  AND id <> ?
  AND (? IS NULL OR ends_at IS NULL OR ends_at >= ?)
  AND (? IS NULL OR starts_at IS NULL OR starts_at <= ?)`,
		productID, adminPromotionTypeLimitedPrice, excludePromotionID,
		startsAt, startsAt,
		endsAt, endsAt,
	).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

func promotionConflictBound(current sql.NullTime, next any, hasNext bool) any {
	if hasNext {
		return next
	}
	if current.Valid {
		return current.Time
	}
	return nil
}

func promotionProductIDsBestEffort(ctx context.Context, db *sql.DB, promotionID int64) []int64 {
	if promotionID <= 0 {
		return nil
	}
	var productID int64
	if err := db.QueryRowContext(ctx, "SELECT product_id FROM mall_product.promotion_rule WHERE id = ?", promotionID).Scan(&productID); err != nil {
		return nil
	}
	if productID <= 0 {
		return nil
	}
	return []int64{productID}
}

func refreshProductCardSnapshotsBestEffort(ctx context.Context, svcCtx *svc.ServiceContext, productIDs ...int64) {
	db, err := svcCtx.SqlConn.RawDB()
	if err != nil {
		return
	}
	if err := ensureGatewayProductReadTables(ctx, db); err != nil {
		return
	}
	for _, productID := range uniquePositiveInt64s(productIDs) {
		_, _ = rebuildGatewayProductCardSnapshots(ctx, db, productID, 1)
	}
}
