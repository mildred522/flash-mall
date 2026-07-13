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
		db, err := orderDB(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		if err = ensureGatewayMerchantBaseSchema(ctx, db); err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		rows, err := db.QueryContext(ctx, `SELECT id,user_id,merchant_name,contact_phone,status,merchant_id,audit_remark,operator_id,COALESCE(create_time,''),COALESCE(audit_time,'') FROM merchant_apply ORDER BY id DESC LIMIT 100`)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		defer rows.Close()
		items := make([]map[string]any, 0)
		for rows.Next() {
			var id, userID, status, merchantID, operatorID int64
			var name, phone, remark, created, audited string
			if err := rows.Scan(&id, &userID, &name, &phone, &status, &merchantID, &remark, &operatorID, &created, &audited); err != nil {
				fail(ctx, c, consts.StatusBadGateway, err)
				return
			}
			items = append(items, map[string]any{"id": id, "user_id": userID, "merchant_name": name, "contact_phone": phone, "status": status, "merchant_id": merchantID, "audit_remark": remark, "operator_id": operatorID, "create_time": created, "audit_time": audited})
		}
		ok(ctx, c, map[string]any{"items": items})
	}
}

func AdminMerchantApplyAuditHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req adminMerchantApplyAuditReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid merchant audit request"))
			return
		}
		if req.ApplyID <= 0 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "apply_id is required"))
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
