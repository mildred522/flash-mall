package handler

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/common/orderstatus"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

var (
	merchantContactPhonePattern = regexp.MustCompile(`^1[3-9][0-9]{9}$`)
	errMerchantAlreadyActive    = errors.New("active merchant already exists")
)

func MerchantMeHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}
		db, err := orderDB(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order datasource unavailable", err))
			return
		}
		resp, err := loadMerchantMe(ctx, db, identity.UserID)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant profile query failed", err))
			return
		}
		ok(ctx, c, resp)
	}
}

func MerchantApplicationHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}
		db, err := orderDB(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order datasource unavailable", err))
			return
		}
		resp, err := loadLatestMerchantApplication(ctx, db, identity.UserID)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant application query failed", err))
			return
		}
		ok(ctx, c, resp)
	}
}

func MerchantApplyCreateHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}
		var req MerchantApplyReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid merchant apply request"))
			return
		}
		req.MerchantName = strings.TrimSpace(req.MerchantName)
		req.ContactPhone = strings.TrimSpace(req.ContactPhone)
		if req.MerchantName == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "merchant_name is required"))
			return
		}
		if req.ContactPhone != "" && !merchantContactPhonePattern.MatchString(req.ContactPhone) {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "contact_phone is invalid"))
			return
		}
		db, err := orderDB(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order datasource unavailable", err))
			return
		}
		resp, err := createMerchantApply(ctx, db, identity.UserID, req)
		if err != nil {
			if errors.Is(err, errMerchantAlreadyActive) {
				fail(ctx, c, consts.StatusConflict, apperror.New(apperror.CodeConflict, err.Error()))
				return
			}
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant apply failed", err))
			return
		}
		ok(ctx, c, resp)
	}
}

func merchantApplicationStatusText(status int64) string {
	switch status {
	case 0:
		return "pending"
	case 1:
		return "approved"
	case 2:
		return "rejected"
	default:
		return "unknown"
	}
}

func loadLatestMerchantApplication(ctx context.Context, db *sql.DB, userID int64) (MerchantApplicationResp, error) {
	row := db.QueryRowContext(ctx, `
SELECT id, merchant_name, contact_phone, status, merchant_id, audit_remark,
       COALESCE(DATE_FORMAT(create_time, '%Y-%m-%d %H:%i:%s'), ''),
       COALESCE(DATE_FORMAT(audit_time, '%Y-%m-%d %H:%i:%s'), '')
FROM merchant_apply
WHERE user_id = ?
ORDER BY id DESC
LIMIT 1`, userID)

	item := MerchantApplicationItem{}
	if err := row.Scan(&item.ApplyID, &item.MerchantName, &item.ContactPhone, &item.Status,
		&item.MerchantID, &item.AuditRemark, &item.CreateTime, &item.AuditTime); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return MerchantApplicationResp{Application: nil}, nil
		}
		return MerchantApplicationResp{}, err
	}
	item.StatusText = merchantApplicationStatusText(item.Status)
	return MerchantApplicationResp{Application: &item}, nil
}

func MerchantDashboardStatsHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}
		db, merchantID, ready := merchantOrderDB(ctx, c, svcCtx, identity)
		if !ready {
			return
		}
		resp, err := loadMerchantDashboardStats(ctx, db, merchantID)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant dashboard query failed", err))
			return
		}
		ok(ctx, c, resp)
	}
}

func loadMerchantMe(ctx context.Context, db *sql.DB, userID int64) (MerchantMeResp, error) {
	rows, err := db.QueryContext(ctx, `
SELECT m.id, m.name, mu.role, m.status
FROM merchant_user mu
JOIN merchant m ON m.id = mu.merchant_id
WHERE mu.user_id = ? AND mu.status = 1
ORDER BY mu.id ASC`, userID)
	if err != nil {
		return MerchantMeResp{}, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]MerchantMeItem, 0)
	for rows.Next() {
		var item MerchantMeItem
		if err := rows.Scan(&item.MerchantID, &item.Name, &item.Role, &item.Status); err != nil {
			return MerchantMeResp{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return MerchantMeResp{}, err
	}
	return MerchantMeResp{Items: items}, nil
}

func createMerchantApply(ctx context.Context, db *sql.DB, userID int64, req MerchantApplyReq) (MerchantApplyResp, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return MerchantApplyResp{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var activeMerchantCount int64
	if err = tx.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM merchant_user mu
JOIN merchant m ON m.id = mu.merchant_id
WHERE mu.user_id = ? AND mu.status = 1 AND m.status = 1`, userID).Scan(&activeMerchantCount); err != nil {
		return MerchantApplyResp{}, err
	}
	if activeMerchantCount > 0 {
		return MerchantApplyResp{}, errMerchantAlreadyActive
	}

	var applyID int64
	err = tx.QueryRowContext(ctx, `
SELECT id
FROM merchant_apply
WHERE user_id = ? AND status = 0
ORDER BY id DESC
LIMIT 1
FOR UPDATE`, userID).Scan(&applyID)
	if err == nil {
		if err = tx.Commit(); err != nil {
			return MerchantApplyResp{}, err
		}
		return MerchantApplyResp{ApplyID: applyID, Status: "pending"}, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return MerchantApplyResp{}, err
	}
	result, err := tx.ExecContext(ctx, "INSERT INTO merchant_apply (user_id, merchant_name, contact_phone, status) VALUES (?, ?, ?, 0)", userID, req.MerchantName, req.ContactPhone)
	if err != nil {
		return MerchantApplyResp{}, err
	}
	applyID, _ = result.LastInsertId()
	if err = tx.Commit(); err != nil {
		return MerchantApplyResp{}, err
	}
	return MerchantApplyResp{ApplyID: applyID, Status: "pending"}, nil
}

func loadMerchantDashboardStats(ctx context.Context, db *sql.DB, merchantID int64) (MerchantDashboardStatsResp, error) {
	resp := MerchantDashboardStatsResp{MerchantID: merchantID}
	if err := db.QueryRowContext(ctx, `
SELECT COUNT(*),
       COALESCE(SUM(CASE WHEN status IN (1,3,4,5,6) THEN 1 ELSE 0 END), 0),
       COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0)
FROM orders
WHERE merchant_id = ?`, orderstatus.Paid, merchantID).Scan(&resp.OrderCount, &resp.PaidOrderCount, &resp.ShipPendingCount); err != nil {
		return MerchantDashboardStatsResp{}, err
	}
	if err := db.QueryRowContext(ctx, `
SELECT COALESCE(SUM(s.payable_amount_fen), 0)
FROM orders o
JOIN order_price_snapshot s ON s.order_id = o.id
WHERE o.merchant_id = ? AND o.status IN (1,3,4,5,6)`, merchantID).Scan(&resp.SalesAmountFen); err != nil {
		return MerchantDashboardStatsResp{}, err
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM refund_order WHERE merchant_id = ? AND status IN (0,1)", merchantID).Scan(&resp.RefundPending); err != nil {
		return MerchantDashboardStatsResp{}, err
	}
	return resp, nil
}
