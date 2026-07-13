package handler

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/gateway/hertz/internal/svc"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type adminMerchantApplyAuditReq struct {
	ApplyID int64  `json:"apply_id"`
	Approve bool   `json:"approve"`
	Remark  string `json:"remark,omitempty"`
}

func AdminMerchantApplyListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		query, err := adminMerchantApplicationQueryFromRequest(c)
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, err)
			return
		}
		db, err := orderDB(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		if err = ensureGatewayMerchantBaseSchema(ctx, db); err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		resp, err := loadAdminMerchantApplications(ctx, db, query)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		ok(ctx, c, resp)
	}
}

func adminMerchantApplicationQueryFromRequest(c *app.RequestContext) (AdminMerchantApplicationListReq, error) {
	status, err := parseInt64Default(c.Query("status"), -1)
	if err != nil || status < -1 || status > 2 {
		return AdminMerchantApplicationListReq{}, apperror.New(apperror.CodeInvalidArgument, "status must be -1, 0, 1 or 2")
	}
	page, err := parseInt64Default(c.Query("page"), 1)
	if err != nil {
		return AdminMerchantApplicationListReq{}, apperror.New(apperror.CodeInvalidArgument, "page must be numeric")
	}
	pageSize, err := parseInt64Default(c.Query("page_size"), 20)
	if err != nil {
		return AdminMerchantApplicationListReq{}, apperror.New(apperror.CodeInvalidArgument, "page_size must be numeric")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return AdminMerchantApplicationListReq{Status: status, Page: page, PageSize: pageSize}, nil
}

func loadAdminMerchantApplications(ctx context.Context, db *sql.DB, req AdminMerchantApplicationListReq) (AdminMerchantApplicationListResp, error) {
	where := "1=1"
	args := make([]any, 0, 1)
	if req.Status >= 0 {
		where += " AND status = ?"
		args = append(args, req.Status)
	}
	var total int64
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM merchant_apply WHERE "+where, args...).Scan(&total); err != nil {
		return AdminMerchantApplicationListResp{}, err
	}
	queryArgs := append(append([]any{}, args...), req.PageSize, (req.Page-1)*req.PageSize)
	rows, err := db.QueryContext(ctx, `
SELECT id, user_id, merchant_name, contact_phone, status, merchant_id,
       audit_remark, operator_id,
       COALESCE(DATE_FORMAT(create_time, '%Y-%m-%d %H:%i:%s'), ''),
       COALESCE(DATE_FORMAT(audit_time, '%Y-%m-%d %H:%i:%s'), '')
FROM merchant_apply
WHERE `+where+`
ORDER BY CASE WHEN status = 0 THEN 0 ELSE 1 END, id DESC
LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return AdminMerchantApplicationListResp{}, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]AdminMerchantApplicationItem, 0)
	for rows.Next() {
		item := AdminMerchantApplicationItem{}
		if err := rows.Scan(&item.ApplyID, &item.UserID, &item.MerchantName, &item.ContactPhone,
			&item.Status, &item.MerchantID, &item.AuditRemark, &item.OperatorID,
			&item.CreateTime, &item.AuditTime); err != nil {
			return AdminMerchantApplicationListResp{}, err
		}
		item.StatusText = merchantApplicationStatusText(item.Status)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return AdminMerchantApplicationListResp{}, err
	}
	return AdminMerchantApplicationListResp{Items: items, Total: total, Page: req.Page, PageSize: req.PageSize}, nil
}

func AdminMerchantApplyAuditHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req adminMerchantApplyAuditReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid merchant audit request"))
			return
		}
		if err := validateAdminMerchantAuditRequest(req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, err)
			return
		}
		db, err := orderDB(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		if err = ensureGatewayMerchantBaseSchema(ctx, db); err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		defer tx.Rollback()
		var userID, status int64
		var name, phone string
		if err = tx.QueryRowContext(ctx, "SELECT user_id,merchant_name,contact_phone,status FROM merchant_apply WHERE id=? FOR UPDATE", req.ApplyID).Scan(&userID, &name, &phone, &status); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeNotFound, "merchant application not found"))
			} else {
				fail(ctx, c, consts.StatusBadGateway, err)
			}
			return
		}
		if status != 0 {
			fail(ctx, c, consts.StatusConflict, apperror.New(apperror.CodeConflict, "merchant application already audited"))
			return
		}
		operatorID := int64(0)
		if identity, ok := authctx.IdentityFrom(ctx); ok {
			operatorID = identity.UserID
		}
		nextStatus, merchantID := int64(2), int64(0)
		if req.Approve {
			nextStatus = 1
			result, err := tx.ExecContext(ctx, "INSERT INTO merchant (name,owner_user_id,status,contact_phone) VALUES (?, ?, 1, ?)", name, userID, phone)
			if err != nil {
				fail(ctx, c, consts.StatusBadGateway, err)
				return
			}
			merchantID, _ = result.LastInsertId()
			if _, err = tx.ExecContext(ctx, "INSERT INTO merchant_user (merchant_id,user_id,role,status) VALUES (?,?,'owner',1) ON DUPLICATE KEY UPDATE role=VALUES(role),status=VALUES(status)", merchantID, userID); err != nil {
				fail(ctx, c, consts.StatusBadGateway, err)
				return
			}
		}
		if _, err = tx.ExecContext(ctx, "UPDATE merchant_apply SET status=?,merchant_id=?,audit_remark=?,operator_id=?,audit_time=NOW() WHERE id=?", nextStatus, merchantID, strings.TrimSpace(req.Remark), operatorID, req.ApplyID); err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		if err = tx.Commit(); err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		ok(ctx, c, map[string]any{"apply_id": req.ApplyID, "merchant_id": merchantID, "status": nextStatus})
	}
}

func validateAdminMerchantAuditRequest(req adminMerchantApplyAuditReq) error {
	if req.ApplyID <= 0 {
		return apperror.New(apperror.CodeInvalidArgument, "apply_id is required")
	}
	if !req.Approve && strings.TrimSpace(req.Remark) == "" {
		return apperror.New(apperror.CodeInvalidArgument, "remark is required when rejecting")
	}
	return nil
}
