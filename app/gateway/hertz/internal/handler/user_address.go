package handler

import (
	"context"
	"database/sql"
	"strings"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func UserAddressListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, found := authctx.IdentityFrom(ctx)
		if !found || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "user login required"))
			return
		}
		db, err := authDB(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		rows, err := db.QueryContext(ctx, `SELECT id, receiver_name, receiver_phone, province, city, district, detail, is_default FROM mall_auth.user_address WHERE user_id = ? AND status = 1 ORDER BY is_default DESC, id DESC`, identity.UserID)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "user address query failed", err))
			return
		}
		defer func() { _ = rows.Close() }()
		items := make([]UserAddressItem, 0)
		for rows.Next() {
			var item UserAddressItem
			var isDefault int64
			if err := rows.Scan(&item.AddressID, &item.ReceiverName, &item.ReceiverPhone, &item.Province, &item.City, &item.District, &item.Detail, &isDefault); err != nil {
				fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "user address scan failed", err))
				return
			}
			item.IsDefault = isDefault == 1
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "user address query failed", err))
			return
		}
		ok(ctx, c, UserAddressListResp{Items: items})
	}
}

func UserAddressUpsertHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, found := authctx.IdentityFrom(ctx)
		if !found || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "user login required"))
			return
		}
		var req UserAddressUpsertReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid user address request"))
			return
		}
		req.ReceiverName = strings.TrimSpace(req.ReceiverName)
		req.ReceiverPhone = strings.TrimSpace(req.ReceiverPhone)
		req.Detail = strings.TrimSpace(req.Detail)
		if req.ReceiverName == "" || req.ReceiverPhone == "" || req.Detail == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "receiver_name, receiver_phone and detail are required"))
			return
		}
		db, err := authDB(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "user address transaction failed", err))
			return
		}
		defer func() { _ = tx.Rollback() }()
		if req.IsDefault {
			if _, err := tx.ExecContext(ctx, "UPDATE mall_auth.user_address SET is_default = 0 WHERE user_id = ?", identity.UserID); err != nil {
				fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "clear default address failed", err))
				return
			}
		}
		isDefault := int64(0)
		if req.IsDefault {
			isDefault = 1
		}
		addressID, err := saveUserAddress(ctx, tx, identity.UserID, req, isDefault)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		if err := tx.Commit(); err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "commit user address failed", err))
			return
		}
		ok(ctx, c, UserAddressUpsertResp{AddressID: addressID})
	}
}

func authDB(svcCtx *svc.ServiceContext) (*sql.DB, error) {
	if svcCtx.AuthSqlConn == nil {
		return nil, apperror.New(apperror.CodeInternal, "auth datasource is not configured")
	}
	return svcCtx.AuthSqlConn.RawDB()
}

func saveUserAddress(ctx context.Context, tx *sql.Tx, userID int64, req UserAddressUpsertReq, isDefault int64) (int64, error) {
	if req.AddressID > 0 {
		result, err := tx.ExecContext(ctx, `UPDATE mall_auth.user_address SET receiver_name=?, receiver_phone=?, province=?, city=?, district=?, detail=?, is_default=? WHERE id=? AND user_id=? AND status=1`, req.ReceiverName, req.ReceiverPhone, req.Province, req.City, req.District, req.Detail, isDefault, req.AddressID, userID)
		if err != nil {
			return 0, apperror.Wrap(apperror.CodeInternal, "update user address failed", err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return 0, apperror.Wrap(apperror.CodeInternal, "inspect user address update failed", err)
		}
		if rows == 0 {
			return 0, apperror.New(apperror.CodeNotFound, "user address not found")
		}
		return req.AddressID, nil
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO mall_auth.user_address (user_id, receiver_name, receiver_phone, province, city, district, detail, is_default) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, userID, req.ReceiverName, req.ReceiverPhone, req.Province, req.City, req.District, req.Detail, isDefault)
	if err != nil {
		return 0, apperror.Wrap(apperror.CodeInternal, "create user address failed", err)
	}
	addressID, err := result.LastInsertId()
	if err != nil {
		return 0, apperror.Wrap(apperror.CodeInternal, "read user address id failed", err)
	}
	return addressID, nil
}
