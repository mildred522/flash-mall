package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"flash-mall/app/entry/api/internal/svc"
	"flash-mall/app/entry/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

const adminPromotionTypeLimitedPrice = "LIMITED_PRICE"

func AdminPromotionListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminPromotionListReq
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
		if req.ProductId > 0 {
			where += " AND pr.product_id = ?"
			args = append(args, req.ProductId)
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
		if keyword := strings.TrimSpace(req.Keyword); keyword != "" {
			where += " AND p.name LIKE ?"
			args = append(args, "%"+keyword+"%")
		}

		var total int64
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM mall_product.promotion_rule pr LEFT JOIN mall_product.product p ON p.id = pr.product_id WHERE %s", where)
		if err := db.QueryRowContext(r.Context(), countQuery, args...).Scan(&total); err != nil {
			logx.WithContext(r.Context()).Errorf("admin promotion count query failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		query := fmt.Sprintf(`
SELECT pr.id, pr.product_id, COALESCE(p.name, ''), COALESCE(p.origin_price_fen, 0), COALESCE(p.sale_price_fen, 0),
       pr.type, pr.discount_value, pr.threshold_amount, pr.starts_at, pr.ends_at, pr.status
FROM mall_product.promotion_rule pr
LEFT JOIN mall_product.product p ON p.id = pr.product_id
WHERE %s
ORDER BY pr.id DESC
LIMIT ? OFFSET ?`, where)
		queryArgs := append(append([]any{}, args...), req.PageSize, offset)
		rows, err := db.QueryContext(r.Context(), query, queryArgs...)
		if err != nil {
			logx.WithContext(r.Context()).Errorf("admin promotion list query failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer func() { _ = rows.Close() }()

		items := make([]types.AdminPromotionItem, 0)
		for rows.Next() {
			var item types.AdminPromotionItem
			var startsAt, endsAt sql.NullTime
			if err := rows.Scan(
				&item.PromotionId,
				&item.ProductId,
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
				logx.WithContext(r.Context()).Errorf("admin promotion list scan failed: %v", err)
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			item.StartsAt = formatAdminPromotionTime(startsAt)
			item.EndsAt = formatAdminPromotionTime(endsAt)
			item.StatusText = adminPromotionStatusText(item.Status)
			item.EffectStatus, item.EffectStatusText = adminPromotionEffectStatus(item.Status, startsAt, endsAt)
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			logx.WithContext(r.Context()).Errorf("admin promotion list rows failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		httpx.OkJsonCtx(r.Context(), w, types.AdminPromotionListResp{Items: items, Total: total})
	}
}

func AdminPromotionDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminPromotionDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if req.PromotionId <= 0 {
			writeBadRequest(w, "promotion_id required")
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		item, err := adminPromotionDetail(r.Context(), db, req.PromotionId)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeNotFound(w, "promotion not found")
				return
			}
			logx.WithContext(r.Context()).Errorf("admin promotion detail query failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, item)
	}
}

func AdminPromotionCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminPromotionCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.Type = normalizeAdminPromotionType(req.Type)
		if req.ProductId <= 0 || req.DiscountValue <= 0 || req.ThresholdAmount < 0 {
			writeBadRequest(w, "product_id, positive discount_value and non-negative threshold_amount are required")
			return
		}
		if req.Type != adminPromotionTypeLimitedPrice {
			writeBadRequest(w, "unsupported promotion type")
			return
		}
		if req.Status == 0 {
			req.Status = 1
		}
		if req.Status != 1 && req.Status != 2 {
			writeBadRequest(w, "status must be 1 or 2")
			return
		}
		startsAt, endsAt, ok := parseAdminPromotionWindow(w, req.StartsAt, req.EndsAt)
		if !ok {
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		exists, err := adminDBProductExists(r.Context(), db, req.ProductId)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if !exists {
			recordAdminAuditFailure(r, svcCtx, adminAuditPromotionCreated, fmt.Sprintf("product:%d reason:%s", req.ProductId, adminAuditReasonProductNotFound))
			writeNotFound(w, "product not found")
			return
		}
		validDiscount, err := adminPromotionDiscountWithinSale(r.Context(), db, req.ProductId, req.DiscountValue)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if !validDiscount {
			recordAdminAuditFailure(r, svcCtx, adminAuditPromotionCreated, fmt.Sprintf("product:%d reason:%s", req.ProductId, adminAuditReasonInvalidDiscount))
			writeBadRequest(w, "discount_value must be <= product sale_price_fen")
			return
		}
		if req.Status == 1 {
			conflict, err := adminPromotionHasActiveConflict(r.Context(), db, req.ProductId, 0, startsAt, endsAt)
			if err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			if conflict {
				recordAdminAuditFailure(r, svcCtx, adminAuditPromotionCreated, fmt.Sprintf("product:%d reason:%s", req.ProductId, adminAuditReasonWindowConflict))
				writeConflict(w, "active limited price promotion window overlaps")
				return
			}
		}

		result, err := db.ExecContext(r.Context(), `
INSERT INTO mall_product.promotion_rule (product_id, type, discount_value, threshold_amount, starts_at, ends_at, status)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
			req.ProductId, req.Type, req.DiscountValue, req.ThresholdAmount, startsAt, endsAt, req.Status,
		)
		if err != nil {
			logx.WithContext(r.Context()).Errorf("admin promotion create failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		promotionID, _ := result.LastInsertId()
		invalidateAdminCatalogCache(r.Context(), svcCtx)
		recordAdminAuditEvent(r, svcCtx, adminAuditPromotionCreated, fmt.Sprintf("promotion:%d product:%d", promotionID, req.ProductId))
		httpx.OkJsonCtx(r.Context(), w, types.AdminPromotionCreateResp{PromotionId: promotionID})
	}
}

func AdminPromotionUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			PromotionId     int64   `json:"promotion_id"`
			ProductId       *int64  `json:"product_id,optional"`       //nolint:staticcheck // go-zero httpx.Parse uses optional in json tags.
			DiscountValue   *int64  `json:"discount_value,optional"`   //nolint:staticcheck // go-zero httpx.Parse uses optional in json tags.
			ThresholdAmount *int64  `json:"threshold_amount,optional"` //nolint:staticcheck // go-zero httpx.Parse uses optional in json tags.
			StartsAt        *string `json:"starts_at,optional"`        //nolint:staticcheck // go-zero httpx.Parse uses optional in json tags.
			EndsAt          *string `json:"ends_at,optional"`          //nolint:staticcheck // go-zero httpx.Parse uses optional in json tags.
			Status          *int64  `json:"status,optional"`           //nolint:staticcheck // go-zero httpx.Parse uses optional in json tags.
		}
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if req.PromotionId <= 0 {
			writeBadRequest(w, "promotion_id required")
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		setClauses := ""
		args := []any{}
		var parsedStartsAt any
		var parsedEndsAt any
		if req.ProductId != nil {
			if *req.ProductId <= 0 {
				writeBadRequest(w, "product_id required")
				return
			}
			exists, err := adminDBProductExists(r.Context(), db, *req.ProductId)
			if err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			if !exists {
				recordAdminAuditFailure(r, svcCtx, adminPromotionUpdateAuditEvent(req.Status), fmt.Sprintf("promotion:%d product:%d reason:%s", req.PromotionId, *req.ProductId, adminAuditReasonProductNotFound))
				writeNotFound(w, "product not found")
				return
			}
			setClauses, args = appendAdminPromotionSet(setClauses, args, "product_id = ?", *req.ProductId)
		}
		if req.DiscountValue != nil {
			if *req.DiscountValue <= 0 {
				writeBadRequest(w, "discount_value must be positive")
				return
			}
			setClauses, args = appendAdminPromotionSet(setClauses, args, "discount_value = ?", *req.DiscountValue)
		}
		if req.ThresholdAmount != nil {
			if *req.ThresholdAmount < 0 {
				writeBadRequest(w, "threshold_amount must be non-negative")
				return
			}
			setClauses, args = appendAdminPromotionSet(setClauses, args, "threshold_amount = ?", *req.ThresholdAmount)
		}
		if req.StartsAt != nil {
			startsAt, parseErr := parseAdminPromotionTime(*req.StartsAt)
			if parseErr != nil {
				writeBadRequest(w, parseErr.Error())
				return
			}
			parsedStartsAt = startsAt
			setClauses, args = appendAdminPromotionSet(setClauses, args, "starts_at = ?", startsAt)
		}
		if req.EndsAt != nil {
			endsAt, parseErr := parseAdminPromotionTime(*req.EndsAt)
			if parseErr != nil {
				writeBadRequest(w, parseErr.Error())
				return
			}
			parsedEndsAt = endsAt
			setClauses, args = appendAdminPromotionSet(setClauses, args, "ends_at = ?", endsAt)
		}
		if req.Status != nil {
			if *req.Status != 1 && *req.Status != 2 {
				writeBadRequest(w, "status must be 1 or 2")
				return
			}
			setClauses, args = appendAdminPromotionSet(setClauses, args, "status = ?", *req.Status)
		}
		if setClauses == "" {
			writeBadRequest(w, "no fields to update")
			return
		}
		if req.ProductId != nil || req.DiscountValue != nil {
			finalProductID, finalDiscountValue, found, err := adminPromotionProductDiscount(r.Context(), db, req.PromotionId)
			if err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			if !found {
				recordAdminAuditFailure(r, svcCtx, adminPromotionUpdateAuditEvent(req.Status), fmt.Sprintf("promotion:%d reason:%s", req.PromotionId, adminAuditReasonNotFound))
				writeNotFound(w, "promotion not found")
				return
			}
			if req.ProductId != nil {
				finalProductID = *req.ProductId
			}
			if req.DiscountValue != nil {
				finalDiscountValue = *req.DiscountValue
			}
			validDiscount, err := adminPromotionDiscountWithinSale(r.Context(), db, finalProductID, finalDiscountValue)
			if err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			if !validDiscount {
				recordAdminAuditFailure(r, svcCtx, adminPromotionUpdateAuditEvent(req.Status), fmt.Sprintf("promotion:%d reason:%s", req.PromotionId, adminAuditReasonInvalidDiscount))
				writeBadRequest(w, "discount_value must be <= product sale_price_fen")
				return
			}
		}
		var currentStartsAt, currentEndsAt sql.NullTime
		hasCurrentWindow := false
		if req.ProductId != nil || req.Status != nil || req.StartsAt != nil || req.EndsAt != nil {
			finalProductID, finalStatus, found, err := adminPromotionProductStatusWindow(r.Context(), db, req.PromotionId, &currentStartsAt, &currentEndsAt)
			if err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			if !found {
				recordAdminAuditFailure(r, svcCtx, adminPromotionUpdateAuditEvent(req.Status), fmt.Sprintf("promotion:%d reason:%s", req.PromotionId, adminAuditReasonNotFound))
				writeNotFound(w, "promotion not found")
				return
			}
			hasCurrentWindow = true
			if req.ProductId != nil {
				finalProductID = *req.ProductId
			}
			if req.Status != nil {
				finalStatus = *req.Status
			}
			if finalStatus == 1 {
				finalStartsAt := adminPromotionConflictBound(currentStartsAt, parsedStartsAt, req.StartsAt != nil)
				finalEndsAt := adminPromotionConflictBound(currentEndsAt, parsedEndsAt, req.EndsAt != nil)
				conflict, err := adminPromotionHasActiveConflict(r.Context(), db, finalProductID, req.PromotionId, finalStartsAt, finalEndsAt)
				if err != nil {
					httpx.ErrorCtx(r.Context(), w, err)
					return
				}
				if conflict {
					recordAdminAuditFailure(r, svcCtx, adminPromotionUpdateAuditEvent(req.Status), fmt.Sprintf("promotion:%d reason:%s", req.PromotionId, adminAuditReasonWindowConflict))
					writeConflict(w, "active limited price promotion window overlaps")
					return
				}
			}
		}
		if req.StartsAt != nil || req.EndsAt != nil {
			if !hasCurrentWindow {
				err := db.QueryRowContext(r.Context(), "SELECT starts_at, ends_at FROM mall_product.promotion_rule WHERE id = ?", req.PromotionId).Scan(&currentStartsAt, &currentEndsAt)
				if errors.Is(err, sql.ErrNoRows) {
					recordAdminAuditFailure(r, svcCtx, adminPromotionUpdateAuditEvent(req.Status), fmt.Sprintf("promotion:%d reason:%s", req.PromotionId, adminAuditReasonNotFound))
					writeNotFound(w, "promotion not found")
					return
				}
				if err != nil {
					httpx.ErrorCtx(r.Context(), w, err)
					return
				}
			}
			if !adminPromotionWindowIsValid(currentStartsAt, currentEndsAt, parsedStartsAt, req.StartsAt != nil, parsedEndsAt, req.EndsAt != nil) {
				recordAdminAuditFailure(r, svcCtx, adminPromotionUpdateAuditEvent(req.Status), fmt.Sprintf("promotion:%d reason:%s", req.PromotionId, adminAuditReasonInvalidWindow))
				writeBadRequest(w, "ends_at must be after starts_at")
				return
			}
		}

		query := fmt.Sprintf("UPDATE mall_product.promotion_rule SET %s WHERE id = ?", setClauses)
		args = append(args, req.PromotionId)
		result, err := db.ExecContext(r.Context(), query, args...)
		if err != nil {
			logx.WithContext(r.Context()).Errorf("admin promotion update failed: %v", err)
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
			if err := db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM mall_product.promotion_rule WHERE id = ?", req.PromotionId).Scan(&exists); err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			if exists == 0 {
				recordAdminAuditFailure(r, svcCtx, adminPromotionUpdateAuditEvent(req.Status), fmt.Sprintf("promotion:%d reason:%s", req.PromotionId, adminAuditReasonNotFound))
				writeNotFound(w, "promotion not found")
				return
			}
		}
		invalidateAdminCatalogCache(r.Context(), svcCtx)
		recordAdminAuditEvent(r, svcCtx, adminPromotionUpdateAuditEvent(req.Status), fmt.Sprintf("promotion:%d", req.PromotionId))
		httpx.OkJsonCtx(r.Context(), w, map[string]any{"ok": true})
	}
}

func adminPromotionDetail(ctx context.Context, q adminQueryRower, promotionID int64) (types.AdminPromotionItem, error) {
	var item types.AdminPromotionItem
	var startsAt, endsAt sql.NullTime
	err := q.QueryRowContext(ctx, `
SELECT pr.id, pr.product_id, COALESCE(p.name, ''), COALESCE(p.origin_price_fen, 0), COALESCE(p.sale_price_fen, 0),
       pr.type, pr.discount_value, pr.threshold_amount, pr.starts_at, pr.ends_at, pr.status
FROM mall_product.promotion_rule pr
LEFT JOIN mall_product.product p ON p.id = pr.product_id
WHERE pr.id = ?`, promotionID).Scan(
		&item.PromotionId,
		&item.ProductId,
		&item.ProductName,
		&item.OriginPriceFen,
		&item.SalePriceFen,
		&item.Type,
		&item.DiscountValue,
		&item.ThresholdAmount,
		&startsAt,
		&endsAt,
		&item.Status,
	)
	if err != nil {
		return types.AdminPromotionItem{}, err
	}
	item.StartsAt = formatAdminPromotionTime(startsAt)
	item.EndsAt = formatAdminPromotionTime(endsAt)
	item.StatusText = adminPromotionStatusText(item.Status)
	item.EffectStatus, item.EffectStatusText = adminPromotionEffectStatus(item.Status, startsAt, endsAt)
	return item, nil
}

func normalizeAdminPromotionType(value string) string {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return adminPromotionTypeLimitedPrice
	}
	return value
}

func parseAdminPromotionWindow(w http.ResponseWriter, startsAtValue, endsAtValue string) (any, any, bool) {
	startsAt, err := parseAdminPromotionTime(startsAtValue)
	if err != nil {
		writeBadRequest(w, err.Error())
		return nil, nil, false
	}
	endsAt, err := parseAdminPromotionTime(endsAtValue)
	if err != nil {
		writeBadRequest(w, err.Error())
		return nil, nil, false
	}
	startTime, startOK := startsAt.(time.Time)
	endTime, endOK := endsAt.(time.Time)
	if startOK && endOK && endTime.Before(startTime) {
		writeBadRequest(w, "ends_at must be after starts_at")
		return nil, nil, false
	}
	return startsAt, endsAt, true
}

func parseAdminPromotionTime(value string) (any, error) {
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

func adminPromotionWindowIsValid(currentStartsAt, currentEndsAt sql.NullTime, nextStartsAt any, hasNextStartsAt bool, nextEndsAt any, hasNextEndsAt bool) bool {
	startsAt := adminPromotionEffectiveTime(currentStartsAt, nextStartsAt, hasNextStartsAt)
	endsAt := adminPromotionEffectiveTime(currentEndsAt, nextEndsAt, hasNextEndsAt)
	return startsAt == nil || endsAt == nil || !endsAt.Before(*startsAt)
}

func adminPromotionEffectiveTime(current sql.NullTime, next any, hasNext bool) *time.Time {
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

func formatAdminPromotionTime(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format("2006-01-02 15:04:05")
}

func appendAdminPromotionSet(setClauses string, args []any, clause string, value any) (string, []any) {
	if setClauses != "" {
		setClauses += ", "
	}
	setClauses += clause
	args = append(args, value)
	return setClauses, args
}

func adminDBProductExists(ctx context.Context, db *sql.DB, productID int64) (bool, error) {
	var exists int64
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mall_product.product WHERE id = ?", productID).Scan(&exists); err != nil {
		return false, err
	}
	return exists > 0, nil
}

func adminPromotionProductStatusWindow(ctx context.Context, db *sql.DB, promotionID int64, startsAt, endsAt *sql.NullTime) (int64, int64, bool, error) {
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

func adminPromotionProductDiscount(ctx context.Context, db *sql.DB, promotionID int64) (int64, int64, bool, error) {
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

func adminPromotionDiscountWithinSale(ctx context.Context, db *sql.DB, productID, discountValue int64) (bool, error) {
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

func adminPromotionHasActiveConflict(ctx context.Context, db *sql.DB, productID, excludePromotionID int64, startsAt, endsAt any) (bool, error) {
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

func adminPromotionConflictBound(current sql.NullTime, next any, hasNext bool) any {
	if hasNext {
		return next
	}
	if current.Valid {
		return current.Time
	}
	return nil
}

func adminPromotionStatusText(status int64) string {
	switch status {
	case 1:
		return "active"
	case 2:
		return "inactive"
	default:
		return "unknown"
	}
}

func adminPromotionEffectStatus(status int64, startsAt, endsAt sql.NullTime) (string, string) {
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

func adminPromotionUpdateAuditEvent(status *int64) string {
	if status == nil {
		return adminAuditPromotionUpdated
	}
	if *status == 1 {
		return adminAuditPromotionEnabled
	}
	return adminAuditPromotionDisabled
}
